package vectordatabase_pgvector_document_count

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
)

const (
	Author       = "Ethan Tan"
	Organisation = "Flomation"
	Name         = "Count Documents"
	Description  = "Count the documents in the store, with an optional metadata filter"
	Website      = "https://www.flomation.co"
	Icon         = "database+hashtag"
	Date         = "13/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Database Host", Placeholder: "db.example.com or 192.168.1.20 — hostname or IP, no scheme", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "5432"},
	{Name: "database", Type: core.ConnectionTypeString, Label: "Database", Placeholder: "vectordb", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "postgres", Required: true},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "Database password", Required: true},
	{Name: "ssl_mode", Type: core.ConnectionTypeString, Label: "SSL Mode", Placeholder: "disable", Options: pgvector.SSLModeOptions},
	{Name: "schema", Type: core.ConnectionTypeString, Label: "Schema", Placeholder: "public"},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table", Placeholder: "documents", Required: true},
	{Name: "metadata_column", Type: core.ConnectionTypeString, Label: "Metadata Column", Placeholder: "Leave blank to work it out from the table"},
	{Name: "metadata_filter", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata Filter", Placeholder: "Only count documents whose metadata matches every pair"},
	{Name: "metadata_filter_json", Type: core.ConnectionTypeObject, Label: "Advanced Metadata Filter (JSON)", Placeholder: `{"source": {"eq": "handbook"}, "page": {"gt": 3}}`},
	{Name: "collection", Type: core.ConnectionTypeString, Label: "Collection", Placeholder: "Optional — a named sub-set of the table to work within"},
	{Name: "collection_table", Type: core.ConnectionTypeString, Label: "Collection Table", Placeholder: "flomation_vector_collections — where collection names are recorded"},
}

var Outputs = [...]core.Connection{
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Document Count"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := pgvector.GetAuth(inputs)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if auth.Table == "" {
		return pgvector.Failf("Table is required — pick the table you want to count the documents in")
	}
	relation, err := pgvector.QuoteRelation(auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	advanced, err := advancedFilterJSON(core.FindConnection("metadata_filter_json", inputs))
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	ctx, cancel := pgvector.Context(flow)
	defer cancel()

	db, err := pgvector.OpenConn(ctx, auth)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	defer db.Close()

	cols, err := resolveMetadataColumn(ctx, db, auth.Schema, auth.Table,
		pgvector.OptionalString(core.FindConnection("metadata_column", inputs)))
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	collection := pgvector.GetCollection(inputs)
	scope, err := collection.ResolveForRead(ctx, db, auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if collection.Active() && !scope.Exists {
		// A collection that was never written to holds nothing — that is a count
		// of zero, not a failure.
		return pgvector.OK(map[string]interface{}{
			"count":  0,
			"result": map[string]interface{}{"count": 0, "schema": auth.Schema, "table": auth.Table, "collection": collection.Name},
		}, fmt.Sprintf("There's no collection named %q in %s.%s yet, so it has 0 documents.", collection.Name, auth.Schema, auth.Table)), nil
	}

	var where []string
	var args []interface{}
	if scope.Exists {
		clause, err := scope.ReadClause(&args)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		where = append(where, clause)
	}

	filter, err := pgvector.BuildFilter(cols,
		[]*core.Connection{core.FindConnection("metadata_filter", inputs)}, advanced, len(args))
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if filter.SQL != "" {
		where = append(where, filter.SQL)
		args = append(args, filter.Args...)
	}

	// Identifiers only, plus BuildFilter's own bound placeholders — nothing the
	// operator typed reaches the SQL text.
	query := "SELECT count(*) FROM " + relation
	if len(where) > 0 {
		query += " WHERE " + strings.Join(where, " AND ")
	}

	var count int
	if err := db.QueryRowContext(ctx, query, args...).Scan(&count); err != nil {
		return pgvector.Fail(auth, err)
	}

	table := auth.Schema + "." + auth.Table
	filtered := filter.SQL != ""
	summary := fmt.Sprintf("%s has %s.", table, documents(count))
	if filtered {
		summary = fmt.Sprintf("%s has %s matching the filter.", table, documents(count))
	}

	return pgvector.OK(map[string]interface{}{
		"count": count,
		"result": map[string]interface{}{
			"count":    count,
			"schema":   auth.Schema,
			"table":    auth.Table,
			"filtered": filtered,
		},
	}, summary), nil
}

// documents renders the count the way the summary line reads it aloud.
func documents(n int) string {
	if n == 1 {
		return "1 document"
	}
	return fmt.Sprintf("%d documents", n)
}

// advancedFilterJSON renders the Advanced Metadata Filter as the JSON text
// BuildFilter parses.
//
// The editor stores an Object input as its JSON source text, but a ${...}
// reference to an upstream step can land a real map here instead, and
// Connection.String() renders a map with Go's own syntax (map[page:3]) rather
// than JSON — which BuildFilter would then reject as malformed. Marshalling it
// back to JSON makes both routes behave the same.
func advancedFilterJSON(c *core.Connection) (string, error) {
	if c == nil || c.Value == nil {
		return "", nil
	}
	if s, ok := c.Value.(string); ok {
		return strings.TrimSpace(s), nil
	}
	b, err := json.Marshal(c.Value)
	if err != nil {
		return "", fmt.Errorf(
			"couldn't read the Advanced Metadata Filter — it should look like " +
				`{"source": {"eq": "handbook"}, "page": {"gt": 3}}`)
	}
	return string(b), nil
}

// resolveMetadataColumn finds the jsonb column the filter runs against.
//
// The shared ResolveColumns is deliberately not used here: it insists on a
// vector column, because every other action in this node needs one. A count does
// not, and refusing to count the rows of a plain table would be an invented
// restriction — so this resolves the one column the action actually uses, and
// leaves it empty when the table has none (BuildFilter says so if a filter was
// set, and an unfiltered count works regardless).
func resolveMetadataColumn(ctx context.Context, db *sql.DB, schema, table, explicit string) (pgvector.ColumnSet, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.attname, t.typname
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
		return pgvector.ColumnSet{}, err
	}
	defer rows.Close()

	types := map[string]string{}
	var names []string
	for rows.Next() {
		var name, udt string
		if err := rows.Scan(&name, &udt); err != nil {
			return pgvector.ColumnSet{}, err
		}
		types[name] = udt
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		return pgvector.ColumnSet{}, err
	}
	if len(names) == 0 {
		return pgvector.ColumnSet{}, fmt.Errorf(
			"table %s.%s doesn't exist, or this user can't see it", schema, table)
	}

	chosen := ""
	switch {
	case explicit != "":
		udt, ok := types[explicit]
		if !ok {
			return pgvector.ColumnSet{}, fmt.Errorf(
				"the metadata column %q doesn't exist on %s.%s — available columns are: %s",
				explicit, schema, table, strings.Join(names, ", "))
		}
		if udt != "jsonb" && udt != "json" {
			return pgvector.ColumnSet{}, fmt.Errorf(
				"column %q on %s.%s is a %s, not jsonb — metadata has to live in a jsonb column",
				explicit, schema, table, udt)
		}
		chosen = explicit
	default:
		// Same candidates, in the same order, as the shared auto-detect: the
		// first is what LangChain and n8n write.
		for _, candidate := range []string{"metadata", "meta", "cmetadata"} {
			if udt, ok := types[candidate]; ok && (udt == "jsonb" || udt == "json") {
				chosen = candidate
				break
			}
		}
	}

	if chosen == "" {
		return pgvector.ColumnSet{}, nil
	}
	quoted, err := pgvector.QuoteIdent(chosen)
	if err != nil {
		return pgvector.ColumnSet{}, err
	}
	return pgvector.ColumnSet{Metadata: chosen, QMetadata: quoted}, nil
}
