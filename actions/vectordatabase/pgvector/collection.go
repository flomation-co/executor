package pgvector

import (
	"context"
	"database/sql"
	"errors"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	"github.com/lib/pq"
)

// Collections — one physical table partitioned into named sub-sets.
//
// This exists for parity with, and drop-in migration from, n8n's PGVector node,
// whose "Use Collection" option stores every collection in one table and tags
// each row with a collection_id that points at a separate collection table. A
// customer who built their store that way in n8n has a table our node would
// otherwise read straight through — returning rows from every collection at once
// — so we speak the same schema:
//
//	<collection table>: (uuid uuid PK, name varchar, cmetadata jsonb)
//	<document table>:    ... + collection_id uuid
//
// It is entirely optional. Leave Collection blank and none of this runs; the
// node operates on the whole table exactly as before. That keeps the common case
// — one table, no collections — as simple as it should be for the audience.
//
// Two of n8n's collection bugs are fixed here: the get-or-create is race-safe
// (n8n's is a bare SELECT-then-INSERT with no unique constraint, so two
// concurrent runs create duplicate collections), and the column/constraint
// bootstrap is driven by catalog checks rather than by string-matching the word
// "already exists" in an error message.

const (
	// DefaultCollectionTable is where collection names map to ids. An operator
	// migrating from n8n sets this to their n8n table (n8n_vector_collections).
	DefaultCollectionTable = "flomation_vector_collections"

	// collectionIDColumn is the column added to the document table. The name
	// matches n8n/LangChain so their tables interoperate unchanged.
	collectionIDColumn = "collection_id"
)

// CollectionInputs documents the two-input collection block, re-declared
// verbatim in each action that supports collections (the manifest generator
// AST-parses the Inputs literal and cannot follow a shared var).
var CollectionInputs = []core.Connection{
	{Name: "collection", Type: core.ConnectionTypeString, Label: "Collection", Placeholder: "Optional — a named sub-set of the table to work within"},
	{Name: "collection_table", Type: core.ConnectionTypeString, Label: "Collection Table", Placeholder: "flomation_vector_collections — where collection names are recorded"},
}

// Collection is the collection block as the operator filled it in.
type Collection struct {
	Name  string
	Table string
}

// GetCollection reads the collection block. An empty Name means "no collection".
func GetCollection(inputs []*core.Connection) Collection {
	c := Collection{
		Name:  OptionalString(core.FindConnection("collection", inputs)),
		Table: OptionalString(core.FindConnection("collection_table", inputs)),
	}
	if c.Table == "" {
		c.Table = DefaultCollectionTable
	}
	return c
}

// Active reports whether the operator asked to scope to a collection.
func (c Collection) Active() bool { return c.Name != "" }

// Scope is a resolved collection: its id, and whether it exists at all.
type Scope struct {
	Active bool
	Exists bool   // a named collection that has never been written to does not exist yet
	ID     string // the collection uuid, when Exists
}

// ResolveForRead looks up (never creates) the collection for a read path.
//
// A read against a collection that does not exist yet is not an error — it is
// simply empty — so Exists=false is returned rather than a failure. A table that
// has no collection_id column at all, though, cannot answer a collection-scoped
// question, and that IS reported: the operator pointed a collection query at a
// table that was never set up for collections.
func (c Collection) ResolveForRead(ctx context.Context, db *sql.DB, schema, docTable string) (Scope, error) {
	if !c.Active() {
		return Scope{Active: false}, nil
	}
	if err := c.requireCollectionColumn(ctx, db, schema, docTable); err != nil {
		return Scope{}, err
	}

	qColl, err := QuoteRelation(schema, c.Table)
	if err != nil {
		return Scope{}, err
	}
	if !c.collectionTableExists(ctx, db, schema) {
		// No collection table means no collection has ever been created — the
		// scoped read is empty, not broken.
		return Scope{Active: true, Exists: false}, nil
	}

	var id string
	err = db.QueryRowContext(ctx, `SELECT uuid FROM `+qColl+` WHERE name = $1 LIMIT 1`, c.Name).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return Scope{Active: true, Exists: false}, nil
	}
	if err != nil {
		return Scope{}, err
	}
	return Scope{Active: true, Exists: true, ID: id}, nil
}

// ResolveForWrite gets-or-creates the collection, provisioning the collection
// table and the document table's collection_id column as needed, and returns
// the collection id to stamp onto every written row.
func (c Collection) ResolveForWrite(ctx context.Context, db *sql.DB, schema, docTable string) (string, error) {
	qColl, err := QuoteRelation(schema, c.Table)
	if err != nil {
		return "", err
	}
	qDoc, err := QuoteRelation(schema, docTable)
	if err != nil {
		return "", err
	}

	// The collection table. gen_random_uuid is built into Postgres 13+; the
	// node's other DDL assumes the same.
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS `+qColl+` (
		uuid uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
		name character varying NOT NULL,
		cmetadata jsonb)`); err != nil {
		return "", err
	}
	// The unique index is what makes the get-or-create race-safe and is the bug
	// n8n never fixed. Best-effort: an n8n table that already holds duplicate
	// names cannot take it, and the get-or-create below still works without it
	// (just without the concurrency guarantee) — so a failure here is ignored.
	qIdx, err := QuoteIdent(sanitiseIndexName(c.Table + "_name_key"))
	if err == nil {
		_, _ = db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS `+qIdx+` ON `+qColl+` (name)`)
	}

	// The document table's collection_id column.
	qCollCol, err := QuoteIdent(collectionIDColumn)
	if err != nil {
		return "", err
	}
	if _, err := db.ExecContext(ctx, `ALTER TABLE `+qDoc+` ADD COLUMN IF NOT EXISTS `+qCollCol+` uuid`); err != nil {
		return "", err
	}

	return getOrCreateCollection(ctx, db, qColl, c.Name)
}

// getOrCreateCollection resolves a collection id, creating it if absent, and is
// safe to run concurrently with another copy of itself whether or not the
// unique index exists: a lost insert race comes back as a unique violation and
// re-reads the winner's row.
func getOrCreateCollection(ctx context.Context, db *sql.DB, qColl, name string) (string, error) {
	var id string
	err := db.QueryRowContext(ctx, `SELECT uuid FROM `+qColl+` WHERE name = $1 LIMIT 1`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", err
	}

	err = db.QueryRowContext(ctx, `INSERT INTO `+qColl+` (name) VALUES ($1) RETURNING uuid`, name).Scan(&id)
	if err == nil {
		return id, nil
	}
	var pe *pq.Error
	if errors.As(err, &pe) && pe.Code == "23505" {
		if e2 := db.QueryRowContext(ctx, `SELECT uuid FROM `+qColl+` WHERE name = $1 LIMIT 1`, name).Scan(&id); e2 == nil {
			return id, nil
		}
	}
	return "", err
}

// ReadClause appends the collection filter's bind arg to args and returns the
// SQL fragment `"collection_id" = $N`. Call it only when the scope Exists.
func (s Scope) ReadClause(args *[]interface{}) (string, error) {
	q, err := QuoteIdent(collectionIDColumn)
	if err != nil {
		return "", err
	}
	*args = append(*args, s.ID)
	return q + " = $" + strconv.Itoa(len(*args)), nil
}

// QuotedIDColumn is the quoted collection_id column, for write paths that name
// it in an INSERT column list.
func QuotedIDColumn() string {
	q, _ := QuoteIdent(collectionIDColumn)
	return q
}

// requireCollectionColumn fails with an operator-readable message when the
// document table has no collection_id column.
func (c Collection) requireCollectionColumn(ctx context.Context, db *sql.DB, schema, docTable string) error {
	cols, err := describeColumns(ctx, db, schema, docTable)
	if err != nil {
		return err
	}
	for _, col := range cols {
		if col.Name == collectionIDColumn {
			return nil
		}
	}
	return errors.New(
		"you asked for a collection, but " + schema + "." + docTable + " has no \"collection_id\" column, so it " +
			"isn't set up for collections — insert into it with a collection first, or clear the Collection field")
}

func (c Collection) collectionTableExists(ctx context.Context, db *sql.DB, schema string) bool {
	var exists bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS (
		SELECT 1 FROM information_schema.tables WHERE table_schema = $1 AND table_name = $2)`,
		schema, c.Table).Scan(&exists)
	return err == nil && exists
}

// sanitiseIndexName trims a candidate index name to something QuoteIdent will
// accept, replacing anything outside the identifier alphabet with underscores.
func sanitiseIndexName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_':
			b.WriteRune(r)
		default:
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" || !(out[0] == '_' || (out[0] >= 'A' && out[0] <= 'Z') || (out[0] >= 'a' && out[0] <= 'z')) {
		out = "c_" + out
	}
	if len(out) > 63 {
		out = out[:63]
	}
	return out
}
