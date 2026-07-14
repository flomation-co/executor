// Package vectordatabase_pgvector_document_list browses the documents in a
// pgvector table.
//
// Every other action in this group is a needle: search for the nearest
// neighbours, fetch one document by ID, delete a match. This one is the
// haystack — it is the only way an operator can see what is actually in their
// knowledge base. n8n's PGVector node has no equivalent at all: you can load
// documents into it and you can retrieve by similarity, but you cannot look.
// When a retrieval step comes back with the wrong passage, "let me just read
// the table" is the first thing anyone wants to do, and needing a DBA and a
// psql prompt to do it is the difference between a flow the operator owns and a
// flow they have to escalate.
package vectordatabase_pgvector_document_list

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "List Documents"
	Description  = "Browse the documents in the store, with an optional metadata filter"
	Website      = "https://www.flomation.co"
	Icon         = "database+list"
	Date         = "13/07/2026"
	Type         = core.ActionTypeAction
)

const defaultLimit = 50

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Database Host", Placeholder: "db.example.com or 192.168.1.20 — hostname or IP, no scheme", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "5432"},
	{Name: "database", Type: core.ConnectionTypeString, Label: "Database", Placeholder: "vectordb", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "postgres", Required: true},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "Database password", Required: true},
	{Name: "ssl_mode", Type: core.ConnectionTypeString, Label: "SSL Mode", Placeholder: "disable", Options: []core.ConnectionOption{{Name: "Disable — no encryption", Value: "disable"}, {Name: "Allow", Value: "allow"}, {Name: "Prefer — encrypt if the server offers it", Value: "prefer"}, {Name: "Require — encrypt, but don't verify the certificate", Value: "require"}, {Name: "Verify CA — encrypt and check the certificate authority", Value: "verify-ca"}, {Name: "Verify Full — encrypt and check the hostname too", Value: "verify-full"}}},
	{Name: "schema", Type: core.ConnectionTypeString, Label: "Schema", Placeholder: "public"},

	{Name: "table", Type: core.ConnectionTypeString, Label: "Table", Placeholder: "documents", Required: true},

	{Name: "id_column", Type: core.ConnectionTypeString, Label: "ID Column", Placeholder: "Leave blank to detect it automatically"},
	{Name: "content_column", Type: core.ConnectionTypeString, Label: "Content Column", Placeholder: "Leave blank to detect it automatically"},
	{Name: "metadata_column", Type: core.ConnectionTypeString, Label: "Metadata Column", Placeholder: "Leave blank to detect it automatically"},
	{Name: "vector_column", Type: core.ConnectionTypeString, Label: "Embedding Column", Placeholder: "Leave blank to detect it automatically"},

	{Name: "metadata_filter", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata Filter", Placeholder: "Only show documents whose metadata matches every pair, e.g. source = handbook"},
	{Name: "metadata_filter_json", Type: core.ConnectionTypeObject, Label: "Advanced Metadata Filter (JSON)", Placeholder: `{"page": {"gt": 3}, "tag": {"in": ["policy","hr"]}}`},

	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Documents to show (default 50, max 1000)"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset", Placeholder: "Skip this many documents first (default 0)"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "Leave empty to sort by ID"},
	{Name: "order_direction", Type: core.ConnectionTypeString, Label: "Sort Direction", Options: []core.ConnectionOption{
		{Name: "Ascending", Value: "asc"},
		{Name: "Descending", Value: "desc"},
	}},
	{Name: "include_vectors", Type: core.ConnectionTypeBoolean, Label: "Include Embeddings", Placeholder: "Return the raw vectors too — thousands of numbers per document"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Fetch every matching document", Placeholder: "Page through the whole table instead of one page"},
	{Name: "collection", Type: core.ConnectionTypeString, Label: "Collection", Placeholder: "Optional — a named sub-set of the table to list within"},
	{Name: "collection_table", Type: core.ConnectionTypeString, Label: "Collection Table", Placeholder: "flomation_vector_collections — where collection names are recorded"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Documents"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total Matching"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := pgvector.GetAuth(inputs)
	if err != nil {
		return pgvector.Failf("%s", err)
	}
	if auth.Table == "" {
		return pgvector.Failf("Table is required — pick the table you want to look inside")
	}

	direction, err := sortDirection(pgvector.OptionalString(core.FindConnection("order_direction", inputs)))
	if err != nil {
		return pgvector.Failf("%s", err)
	}
	offset := pgvector.OptionalInt(core.FindConnection("offset", inputs), 0)
	if offset < 0 {
		offset = 0
	}
	limit := pgvector.Clamp(pgvector.OptionalInt(core.FindConnection("limit", inputs), defaultLimit), defaultLimit, pgvector.MaxRows)
	includeVectors := pgvector.OptionalBool(core.FindConnection("include_vectors", inputs), false)
	returnAll := pgvector.OptionalBool(core.FindConnection("return_all", inputs), false)

	ctx, cancel := pgvector.Context(flow)
	defer cancel()

	db, err := pgvector.OpenConn(ctx, auth)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	defer db.Close()

	cols, err := pgvector.ResolveColumns(ctx, db, auth.Schema, auth.Table, pgvector.ColumnInputs{
		ID:       pgvector.OptionalString(core.FindConnection("id_column", inputs)),
		Content:  pgvector.OptionalString(core.FindConnection("content_column", inputs)),
		Metadata: pgvector.OptionalString(core.FindConnection("metadata_column", inputs)),
		Vector:   pgvector.OptionalString(core.FindConnection("vector_column", inputs)),
	})
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	rel, err := pgvector.QuoteRelation(auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Failf("%s", err)
	}

	orderBy, orderSQL, err := orderClause(ctx, db, auth, cols, pgvector.OptionalString(core.FindConnection("order_by", inputs)), direction)
	if err != nil {
		return pgvector.Failf("%s", err)
	}

	collection := pgvector.GetCollection(inputs)
	scope, err := collection.ResolveForRead(ctx, db, auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if collection.Active() && !scope.Exists {
		return pgvector.OK(map[string]interface{}{
			"results": []map[string]interface{}{}, "count": 0, "total": 0,
			"result": map[string]interface{}{"schema": auth.Schema, "table": auth.Table, "collection": collection.Name, "total": 0},
		}, fmt.Sprintf("There's no collection named %q in %s.%s yet, so there's nothing to list.",
			collection.Name, auth.Schema, auth.Table)), nil
	}

	// The collection filter (if any) binds first, then the metadata filter, then
	// LIMIT/OFFSET take the two slots after both.
	var baseArgs []interface{}
	var whereClauses []string
	if scope.Exists {
		clause, err := scope.ReadClause(&baseArgs)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		whereClauses = append(whereClauses, clause)
	}

	filter, err := pgvector.BuildFilter(
		cols,
		[]*core.Connection{core.FindConnection("metadata_filter", inputs)},
		pgvector.OptionalString(core.FindConnection("metadata_filter_json", inputs)),
		len(baseArgs),
	)
	if err != nil {
		return pgvector.Failf("%s", err)
	}
	if filter.SQL != "" {
		whereClauses = append(whereClauses, filter.SQL)
		baseArgs = append(baseArgs, filter.Args...)
	}

	where := ""
	if len(whereClauses) > 0 {
		where = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// The full match count is what turns a page into a fact: "showing 50 of 8,412"
	// tells the operator their filter is too broad, where a bare 50 tells them
	// nothing at all.
	var total int
	if err := db.QueryRowContext(ctx, "SELECT count(*) FROM "+rel+where, baseArgs...).Scan(&total); err != nil {
		return pgvector.Fail(auth, err)
	}

	selectSQL, projection := selectStatement(rel, where, orderSQL, cols, includeVectors, len(baseArgs))

	// return_all pages at MaxRows regardless of Limit — the operator asked for
	// everything, and a page size of 1 would otherwise burn all 20 pages.
	pageSize, maxPages := limit, 1
	if returnAll {
		pageSize, maxPages = pgvector.MaxRows, pgvector.MaxAllPages
	}

	docs := make([]map[string]interface{}, 0, pageSize)
	for page := 0; page < maxPages; page++ {
		args := make([]interface{}, 0, len(baseArgs)+2)
		args = append(args, baseArgs...)
		args = append(args, pageSize, offset+len(docs))

		batch, err := scanPage(ctx, db, selectSQL, args, projection)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		docs = append(docs, batch...)

		if len(batch) < pageSize {
			break // a short page is the end of the table
		}
	}

	// Anything left behind is either the page cap biting or the operator's own
	// Limit; either way the summary has to say so, because a truncated answer
	// that reads as a complete one is how someone concludes a document isn't in
	// the store when it is.
	fetched := offset + len(docs)
	truncated := fetched < total

	fields := map[string]interface{}{
		"results": docs,
		"count":   len(docs),
		"total":   total,
		"result": map[string]interface{}{
			"schema":          auth.Schema,
			"table":           auth.Table,
			"documents":       docs,
			"count":           len(docs),
			"total":           total,
			"offset":          offset,
			"order_by":        orderBy,
			"order_direction": strings.ToLower(direction),
			"filtered":        filter.SQL != "",
			"truncated":       truncated,
		},
	}
	return pgvector.OK(fields, summarise(auth, len(docs), total, offset, returnAll, truncated, filter.SQL != "", docs)), nil
}

// ---------------------------------------------------------------------------
// Query construction
// ---------------------------------------------------------------------------

// sortDirection maps the input to one of exactly two literals. ASC/DESC cannot
// be bound as a parameter, so the raw string never reaches the SQL — only one of
// these two constants does.
func sortDirection(raw string) (string, error) {
	switch strings.ToLower(raw) {
	case "", "asc":
		return "ASC", nil
	case "desc":
		return "DESC", nil
	}
	return "", fmt.Errorf("%q isn't a sort direction — choose Ascending or Descending", raw)
}

// orderClause resolves Sort By into a quoted ORDER BY fragment.
//
// A column name is an identifier, so it cannot be bound and has to be
// interpolated. Two gates stand in front of that: QuoteIdent rejects anything
// that is not identifier-shaped, and the catalog lookup proves the name is a
// real column on this table — so the only strings that ever reach the SQL text
// are names Postgres itself has just confirmed.
func orderClause(ctx context.Context, db *sql.DB, auth pgvector.Auth, cols pgvector.ColumnSet, raw, direction string) (string, string, error) {
	if raw == "" {
		return cols.ID, cols.QID + " " + direction, nil
	}

	quoted, err := pgvector.QuoteIdent(raw)
	if err != nil {
		return "", "", err
	}
	ok, err := columnExists(ctx, db, auth.Schema, auth.Table, raw)
	if err != nil {
		return "", "", fmt.Errorf("%s", pgvector.Humanise(auth, err))
	}
	if !ok {
		return "", "", fmt.Errorf(
			"there's no column called %q on %s.%s to sort by — column names are case-sensitive, and leaving Sort By empty sorts by %s",
			raw, auth.Schema, auth.Table, cols.ID)
	}

	clause := quoted + " " + direction
	if raw != cols.ID {
		// Paging with OFFSET over a non-unique sort key can show the same row
		// twice and skip another, because Postgres is free to order the ties
		// differently on each page. The ID tiebreaker makes the order total.
		clause += ", " + cols.QID + " ASC"
	}
	return raw, clause, nil
}

func columnExists(ctx context.Context, db *sql.DB, schema, table, column string) (bool, error) {
	var exists bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			  FROM pg_attribute a
			  JOIN pg_class     c ON c.oid = a.attrelid
			  JOIN pg_namespace n ON n.oid = c.relnamespace
			 WHERE n.nspname = $1
			   AND c.relname = $2
			   AND a.attname = $3
			   AND a.attnum  > 0
			   AND NOT a.attisdropped)`, schema, table, column).Scan(&exists)
	return exists, err
}

// selectStatement builds the page query and the field names its columns map to.
//
// Every identifier here is pre-quoted by ResolveColumns or QuoteRelation, and
// the only integers interpolated are placeholder numbers we counted ourselves.
// LIMIT and OFFSET are bound, not interpolated.
func selectStatement(rel, where, orderSQL string, cols pgvector.ColumnSet, includeVectors bool, boundArgs int) (string, []string) {
	selected := []string{cols.QID, cols.QContent}
	projection := []string{"id", "content"}

	if cols.HasMetadata() {
		selected = append(selected, cols.QMetadata)
		projection = append(projection, "metadata")
	}
	if includeVectors {
		selected = append(selected, cols.QVector)
		projection = append(projection, "embedding")
	}

	stmt := "SELECT " + strings.Join(selected, ", ") +
		" FROM " + rel + where +
		" ORDER BY " + orderSQL +
		" LIMIT $" + strconv.Itoa(boundArgs+1) +
		" OFFSET $" + strconv.Itoa(boundArgs+2)
	return stmt, projection
}

// ---------------------------------------------------------------------------
// Rows
// ---------------------------------------------------------------------------

// scanPage reads one page into plain Go values.
//
// The columns are scanned into interface{} rather than typed destinations
// because the node has no idea what they are: an ID is as likely to be a uuid or
// a text slug as a bigint, and a table nobody planned for us is the normal case.
func scanPage(ctx context.Context, db *sql.DB, stmt string, args []interface{}, projection []string) ([]map[string]interface{}, error) {
	rows, err := db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]interface{}
	for rows.Next() {
		raw := make([]interface{}, len(projection))
		dest := make([]interface{}, len(projection))
		for i := range raw {
			dest[i] = &raw[i]
		}
		if err := rows.Scan(dest...); err != nil {
			return nil, err
		}

		doc := make(map[string]interface{}, len(projection))
		for i, field := range projection {
			switch field {
			case "content":
				doc[field] = text(raw[i])
			case "metadata":
				doc[field] = decodeMetadata(raw[i])
			case "embedding":
				vec, err := pgvector.ParseVector(raw[i])
				if err != nil {
					return nil, err
				}
				doc[field] = vec
				doc["dimensions"] = len(vec)
			default:
				doc[field] = scalar(raw[i])
			}
		}
		out = append(out, doc)
	}
	return out, rows.Err()
}

// scalar renders a driver value as something JSON can carry. lib/pq hands back
// []byte for every text-ish type, which marshals to base64 unless it is turned
// back into a string first.
func scalar(v interface{}) interface{} {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

func text(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(t)
	case string:
		return t
	}
	return fmt.Sprintf("%v", v)
}

// decodeMetadata unwraps a jsonb column into a real object, so downstream steps
// can pick ${node.results[0].metadata.source} rather than parse a string.
// Anything that will not decode is passed through as text rather than dropped —
// showing the operator what is really in the column beats hiding it.
func decodeMetadata(v interface{}) interface{} {
	var buf []byte
	switch t := v.(type) {
	case nil:
		return nil
	case []byte:
		buf = t
	case string:
		buf = []byte(t)
	default:
		return v
	}

	var out interface{}
	if err := json.Unmarshal(buf, &out); err != nil {
		return string(buf)
	}
	return out
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

func summarise(auth pgvector.Auth, count int, total int, offset int, returnAll, truncated, filtered bool, docs []map[string]interface{}) string {
	table := auth.Schema + "." + auth.Table
	scope := ""
	if filtered {
		scope = " matching the filter"
	}

	var b strings.Builder
	switch {
	case count == 0 && total == 0:
		fmt.Fprintf(&b, "No documents%s in %s", scope, table)
	case count == 0:
		fmt.Fprintf(&b, "No documents to show — %d document(s)%s in %s, but Offset skips past all of them", total, scope, table)
	case returnAll && truncated:
		fmt.Fprintf(&b,
			"Fetched %d document(s)%s from %s, but stopped at the safety cap of %d pages of %d — %d match in total, "+
				"so this is NOT the whole table. Narrow the filter, or work through it a page at a time with Limit and Offset",
			count, scope, table, pgvector.MaxAllPages, pgvector.MaxRows, total)
	case truncated:
		fmt.Fprintf(&b,
			"Showing %d of %d document(s)%s in %s, starting at %d — raise Offset to see the next page, or tick \"Fetch every matching document\"",
			count, total, scope, table, offset)
	default:
		fmt.Fprintf(&b, "Found %d document(s)%s in %s", count, scope, table)
	}

	if count > 0 {
		if first := pgvector.Preview(text(docs[0]["content"])); first != "" {
			fmt.Fprintf(&b, ". First: %q", first)
		}
	}
	return b.String()
}
