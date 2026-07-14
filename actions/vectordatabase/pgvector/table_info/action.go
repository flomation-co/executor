package vectordatabase_pgvector_table_info

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Inspect Vector Table"
	Description  = "See a table's columns, embedding dimensions, indexes and row count"
	Website      = "https://www.flomation.co"
	Icon         = "database+magnifying-glass"
	Date         = "13/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Database Host", Placeholder: "db.example.com or 192.168.1.20 — hostname or IP, no scheme", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "5432"},
	{Name: "database", Type: core.ConnectionTypeString, Label: "Database", Placeholder: "vectordb", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "postgres", Required: true},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "Database password", Required: true},
	{Name: "ssl_mode", Type: core.ConnectionTypeString, Label: "SSL Mode", Placeholder: "disable", Options: []core.ConnectionOption{{Name: "Disable — no encryption", Value: "disable"}, {Name: "Allow", Value: "allow"}, {Name: "Prefer — encrypt if the server offers it", Value: "prefer"}, {Name: "Require — encrypt, but don't verify the certificate", Value: "require"}, {Name: "Verify CA — encrypt and check the certificate authority", Value: "verify-ca"}, {Name: "Verify Full — encrypt and check the hostname too", Value: "verify-full"}}},
	{Name: "schema", Type: core.ConnectionTypeString, Label: "Schema", Placeholder: "public"},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table", Placeholder: "documents", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "columns", Type: core.ConnectionTypeObject, Label: "Columns"},
	{Name: "vector_column", Type: core.ConnectionTypeString, Label: "Embedding Column"},
	{Name: "content_column", Type: core.ConnectionTypeString, Label: "Content Column"},
	{Name: "metadata_column", Type: core.ConnectionTypeString, Label: "Metadata Column"},
	{Name: "id_column", Type: core.ConnectionTypeString, Label: "ID Column"},
	{Name: "dimensions", Type: core.ConnectionTypeInteger, Label: "Dimensions"},
	{Name: "row_count", Type: core.ConnectionTypeInteger, Label: "Row Count"},
	{Name: "indexes", Type: core.ConnectionTypeObject, Label: "Indexes"},
	{Name: "extension_installed", Type: core.ConnectionTypeBoolean, Label: "pgvector Installed"},
	{Name: "table_size", Type: core.ConnectionTypeString, Label: "Table Size (including indexes)"},
	{Name: "has_vector_index", Type: core.ConnectionTypeBoolean, Label: "Has Vector Index"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Table Info"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// This is the action an operator reaches for when search "isn't working", so it
// has to survive being pointed at a table that is not a vector table at all —
// that is frequently the answer. Nothing here is treated as fatal except being
// unable to read the catalog: a missing extension, a missing embedding column
// and a missing index are all *findings*, reported in tool_result rather than
// raised as errors.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := pgvector.GetAuth(inputs)
	if err != nil {
		return pgvector.Failf("%s", err.Error())
	}
	if auth.Table == "" {
		return pgvector.Failf("Table is required — name the table you want to inspect")
	}
	rel, err := pgvector.QuoteRelation(auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Failf("%s", err.Error())
	}

	ctx, cancel := pgvector.Context(flow)
	defer cancel()

	db, err := pgvector.OpenConn(ctx, auth)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	defer db.Close()

	extension, err := extensionInstalled(ctx, db)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	columns, err := describeTable(ctx, db, auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if len(columns) == 0 {
		return pgvector.Failf(
			"table %s.%s doesn't exist, or this user can't see it. Names are case-sensitive here, so \"Documents\" "+
				"and \"documents\" are different tables", auth.Schema, auth.Table)
	}

	// ResolveColumns is the same detection every other step in this node runs, so
	// running it here means the report says what those steps will actually do —
	// including when it fails, which is the most useful thing this action can
	// tell anyone. When it does fail we still name the embedding column if there
	// is exactly one, because the failure is usually about content or ID.
	resolved, resolveErr := pgvector.ResolveColumns(ctx, db, auth.Schema, auth.Table, pgvector.ColumnInputs{})

	var vectorNames []string
	for _, c := range columns {
		if c.IsVector {
			vectorNames = append(vectorNames, c.Name)
		}
	}

	var idCol, contentCol, metaCol, vectorCol, note string
	if resolveErr == nil {
		idCol, contentCol, metaCol, vectorCol = resolved.ID, resolved.Content, resolved.Metadata, resolved.Vector
	} else {
		if len(vectorNames) == 1 {
			vectorCol = vectorNames[0]
		}
		note = resolveErr.Error()
	}

	dimensions := 0
	for _, c := range columns {
		if c.Name == vectorCol {
			dimensions = c.Dimension
		}
	}

	// A full scan, deliberately. reltuples would be free but is an estimate that
	// reads 0 on a table nobody has ANALYZEd, and "0 documents" sends an operator
	// hunting for a bug that isn't there. This step is an explicit diagnostic; it
	// is allowed to cost a scan.
	var rowCount int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+rel).Scan(&rowCount); err != nil {
		return pgvector.Fail(auth, err)
	}

	// rel is already quoted, so it binds as the exact relation regclass resolves.
	var tableSize string
	if err := db.QueryRowContext(ctx,
		"SELECT pg_size_pretty(pg_total_relation_size($1::regclass))", rel).Scan(&tableSize); err != nil {
		return pgvector.Fail(auth, err)
	}

	indexes, err := describeIndexes(ctx, db, auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	var vectorIndex *indexRow
	for i := range indexes {
		if _, ann := annMethods[indexes[i].Method]; ann {
			vectorIndex = &indexes[i]
			break
		}
	}

	columnsOut := make([]map[string]interface{}, 0, len(columns))
	names := make([]string, 0, len(columns))
	for _, c := range columns {
		columnsOut = append(columnsOut, map[string]interface{}{
			"name":      c.Name,
			"type":      c.Type,
			"dimension": c.Dimension,
		})
		names = append(names, c.Name)
	}

	indexesOut := make([]map[string]interface{}, 0, len(indexes))
	for _, ix := range indexes {
		indexesOut = append(indexesOut, map[string]interface{}{
			"name":       ix.Name,
			"type":       ix.Method,
			"definition": ix.Definition,
			"size":       ix.Size,
		})
	}

	summary := summarise(auth, rowCount, names, vectorNames, vectorCol, dimensions, vectorIndex, extension, note)

	result := map[string]interface{}{
		"schema":              auth.Schema,
		"table":               auth.Table,
		"columns":             columnsOut,
		"vector_column":       vectorCol,
		"content_column":      contentCol,
		"metadata_column":     metaCol,
		"id_column":           idCol,
		"dimensions":          dimensions,
		"row_count":           rowCount,
		"indexes":             indexesOut,
		"extension_installed": extension,
		"table_size":          tableSize,
		"has_vector_index":    vectorIndex != nil,
	}
	if note != "" {
		result["note"] = note
	}

	return pgvector.OK(map[string]interface{}{
		"columns":             columnsOut,
		"vector_column":       vectorCol,
		"content_column":      contentCol,
		"metadata_column":     metaCol,
		"id_column":           idCol,
		"dimensions":          dimensions,
		"row_count":           rowCount,
		"indexes":             indexesOut,
		"extension_installed": extension,
		"table_size":          tableSize,
		"has_vector_index":    vectorIndex != nil,
		"result":              result,
	}, summary), nil
}

// ---------------------------------------------------------------------------
// The report
// ---------------------------------------------------------------------------

// summarise writes the line the operator actually reads. It leads with the
// verdict on the two things that break a search — the embedding column and its
// index — because that is what they came here to find out.
func summarise(auth pgvector.Auth, rowCount int, names, vectorNames []string,
	vectorCol string, dimensions int, vectorIndex *indexRow, extension bool, note string) string {

	var b strings.Builder
	fmt.Fprintf(&b, "%s.%s — %s", auth.Schema, auth.Table, countOf(rowCount, vectorCol != ""))

	switch {
	case len(vectorNames) == 0:
		b.WriteString(", no embedding column, so this isn't a vector table yet")

	case vectorCol == "":
		fmt.Fprintf(&b, ", %d embedding columns (%s) — every other step will need Embedding Column set to say which one to use",
			len(vectorNames), strings.Join(vectorNames, ", "))

	case dimensions > 0:
		fmt.Fprintf(&b, ", %d-dimension embeddings in %q", dimensions, vectorCol)

	default:
		// A bare `vector` column: any length goes in, and pgvector cannot build an
		// ANN index on it. This is exactly what n8n's PGVector node creates, so a
		// table imported from n8n lands here — and it explains a search that is
		// permanently slow no matter how many indexes the operator tries to add.
		fmt.Fprintf(&b, ", embeddings in %q with no fixed size (declared as plain `vector`)", vectorCol)
	}

	// The index verdict — the single most common reason a search is slow.
	if vectorCol != "" {
		switch {
		case vectorIndex != nil:
			fmt.Fprintf(&b, ", %s index present", annMethods[vectorIndex.Method])
			if m := indexMetric(vectorIndex.Definition); m != "" {
				fmt.Fprintf(&b, " (%s)", m)
			}
			b.WriteString(". Searches that use a different distance metric from the index won't use it, so keep them in step")

		case dimensions < 0:
			b.WriteString(", and NO vector index — pgvector can't index a column with no fixed size, so every search " +
				"reads every row. Rebuild the table with Create Table (which sets the size) and then add an index")

		default:
			b.WriteString(", but NO vector index — every search reads every row of the table, which is the usual " +
				"reason search is slow. Add one with the Create Index step")
		}
	}

	fmt.Fprintf(&b, ". Columns: %s.", pgvector.Preview(strings.Join(names, ", ")))

	if !extension {
		b.WriteString(" The pgvector extension isn't installed on this database, so it can't store embeddings at all " +
			"— tick \"Create the extension\" on a Create Table step, or ask your DBA to run: CREATE EXTENSION vector;")
	}
	if note != "" {
		fmt.Fprintf(&b, " Heads-up for the other steps: %s.", strings.TrimSuffix(note, "."))
	}
	return b.String()
}

// countOf words the row count the way the table earns it: rows in a plain
// table, documents once there are embeddings in it.
func countOf(n int, isVectorTable bool) string {
	noun := "row"
	if isVectorTable {
		noun = "document"
	}
	if n != 1 {
		noun += "s"
	}
	return strconv.Itoa(n) + " " + noun
}

// annMethods are pgvector's two index access methods, mapped to the spelling an
// operator will recognise from the docs. Both refuse to build on anything but a
// vector column, so finding one on the table IS finding one on the embeddings.
var annMethods = map[string]string{
	"hnsw":    "HNSW",
	"ivfflat": "IVFFlat",
}

// indexMetric reads the distance metric back out of the index definition. The
// operator class is the thing that has to agree with the search's distance
// metric — get them out of step and the index is simply never used, and the
// search is slow for a reason no amount of staring at the query will reveal.
func indexMetric(definition string) string {
	switch {
	case strings.Contains(definition, "cosine_ops"):
		return "cosine"
	case strings.Contains(definition, "_ip_ops"):
		return "inner product"
	case strings.Contains(definition, "_l2_ops"):
		return "euclidean"
	}
	return ""
}

// ---------------------------------------------------------------------------
// Catalog
// ---------------------------------------------------------------------------

type columnRow struct {
	Name      string
	Type      string // as Postgres spells it: "vector(1024)", "jsonb", "text"
	IsVector  bool
	Dimension int // N for vector(N); -1 for a bare `vector`; 0 for anything else
}

type indexRow struct {
	Name       string
	Method     string // access method: hnsw, ivfflat, btree, gin, …
	Definition string
	Size       string
}

// describeTable reads the column list. format_type is what carries the declared
// dimension — information_schema reports a vector only as "USER-DEFINED".
func describeTable(ctx context.Context, db *sql.DB, schema, table string) ([]columnRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.attname,
		       t.typname,
		       format_type(a.atttypid, a.atttypmod)
		  FROM pg_attribute a
		  JOIN pg_class     c ON c.oid = a.attrelid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		  JOIN pg_type      t ON t.oid = a.atttypid
		 WHERE n.nspname = $1
		   AND c.relname = $2
		   AND a.attnum  > 0
		   AND NOT a.attisdropped
		 ORDER BY a.attnum`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []columnRow
	for rows.Next() {
		var name, udt, formatted string
		if err := rows.Scan(&name, &udt, &formatted); err != nil {
			return nil, err
		}
		out = append(out, columnRow{
			Name:      name,
			Type:      formatted,
			IsVector:  udt == "vector",
			Dimension: vectorDimension(udt, formatted),
		})
	}
	return out, rows.Err()
}

// vectorDimension pulls N out of "vector(1536)". A bare "vector" carries no
// dimension at all, which is a real state a table can be in and one the report
// has to be able to say out loud, so it is -1 rather than an error.
func vectorDimension(udt, formatted string) int {
	if udt != "vector" {
		return 0
	}
	open := strings.IndexByte(formatted, '(')
	if open < 0 || !strings.HasSuffix(formatted, ")") {
		return -1
	}
	n, err := strconv.Atoi(formatted[open+1 : len(formatted)-1])
	if err != nil || n <= 0 {
		return -1
	}
	return n
}

// describeIndexes lists every index on the table with the access method that
// built it and what it costs on disk. pg_get_indexdef is the server's own
// rendering, so it is safe to show and carries the operator class with it.
func describeIndexes(ctx context.Context, db *sql.DB, schema, table string) ([]indexRow, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT i.relname,
		       am.amname,
		       pg_get_indexdef(i.oid),
		       pg_size_pretty(pg_relation_size(i.oid))
		  FROM pg_index     x
		  JOIN pg_class     i  ON i.oid  = x.indexrelid
		  JOIN pg_class     c  ON c.oid  = x.indrelid
		  JOIN pg_namespace n  ON n.oid  = c.relnamespace
		  JOIN pg_am        am ON am.oid = i.relam
		 WHERE n.nspname = $1
		   AND c.relname = $2
		 ORDER BY i.relname`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []indexRow
	for rows.Next() {
		var ix indexRow
		if err := rows.Scan(&ix.Name, &ix.Method, &ix.Definition, &ix.Size); err != nil {
			return nil, err
		}
		out = append(out, ix)
	}
	return out, rows.Err()
}

// extensionInstalled answers the question behind half the errors this node can
// raise: is pgvector actually turned on in this database?
func extensionInstalled(ctx context.Context, db *sql.DB) (bool, error) {
	var installed bool
	err := db.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM pg_extension WHERE extname = $1)", "vector").Scan(&installed)
	return installed, err
}
