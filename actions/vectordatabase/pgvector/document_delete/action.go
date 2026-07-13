// Delete Documents removes rows from a pgvector table, either by ID or by the
// same metadata filter the search actions use.
//
// Two things make this action different from every other one in the node.
//
//   - It is the only one that can destroy work, so it carries the
//     confirm_destructive guard — last input, required, boolean — and reads it
//     with OptionalBool(…, false) so an unresolved ${var.approved} fails closed
//     rather than deleting the table's contents on a typo.
//
//   - A DELETE whose WHERE clause compiles to nothing is not "delete nothing",
//     it is "delete everything". An empty metadata filter is therefore refused
//     outright rather than run; so is an empty ID list. There is no supported
//     way to empty a table from this step, which is deliberate.
package vectordatabase_pgvector_document_delete

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
	"github.com/lib/pq"
)

const (
	Author       = "Ethan Tan"
	Organisation = "Flomation"
	Name         = "Delete Documents"
	Description  = "Remove documents by ID or by a metadata filter"
	Website      = "https://www.flomation.co"
	Icon         = "database+trash"
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
	{Name: "id_column", Type: core.ConnectionTypeString, Label: "ID Column", Placeholder: "Leave blank to work it out from the table"},
	{Name: "content_column", Type: core.ConnectionTypeString, Label: "Content Column", Placeholder: "Leave blank to work it out from the table"},
	{Name: "metadata_column", Type: core.ConnectionTypeString, Label: "Metadata Column", Placeholder: "Leave blank to work it out from the table"},
	{Name: "vector_column", Type: core.ConnectionTypeString, Label: "Embedding Column", Placeholder: "Leave blank to work it out from the table"},
	{
		Name:  "delete_by",
		Type:  core.ConnectionTypeString,
		Label: "Delete",
		Options: []core.ConnectionOption{
			{Name: "Documents with these IDs", Value: "ids"},
			{Name: "Documents matching a metadata filter", Value: "filter"},
		},
	},
	{Name: "ids", Type: core.ConnectionTypeString, Label: "Document IDs", Placeholder: "One or more IDs, comma-separated — or a JSON array", Visible: &core.VisibleWhen{Field: "delete_by", Values: []string{"ids"}}},
	{Name: "metadata_filter", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata Filter", Placeholder: "Only documents whose metadata matches every pair are deleted", Visible: &core.VisibleWhen{Field: "delete_by", Values: []string{"filter"}}},
	{Name: "metadata_filter_json", Type: core.ConnectionTypeObject, Label: "Advanced Metadata Filter (JSON)", Placeholder: `{"source": {"eq": "handbook"}, "page": {"gt": 3}}`, Visible: &core.VisibleWhen{Field: "delete_by", Values: []string{"filter"}}},
	{Name: "collection", Type: core.ConnectionTypeString, Label: "Collection", Placeholder: "Optional — restrict the delete to one named collection"},
	{Name: "collection_table", Type: core.ConnectionTypeString, Label: "Collection Table", Placeholder: "flomation_vector_collections — where collection names are recorded"},
	{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Label: "I understand this permanently deletes data", Placeholder: "Deleted documents cannot be recovered. Tick to allow, or bind a variable such as ${var.approved}", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "deleted", Type: core.ConnectionTypeInteger, Label: "Documents Deleted"},
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
		return pgvector.Failf("Table is required — pick the table to delete documents from")
	}

	// Read before anything is opened: a run that was never approved should not
	// even reach the database.
	if !pgvector.OptionalBool(core.FindConnection("confirm_destructive", inputs), false) {
		return pgvector.Failf(`Tick "I understand this permanently deletes data" to run this step.`)
	}

	mode := pgvector.OptionalString(core.FindConnection("delete_by", inputs))
	if mode == "" {
		mode = "ids"
	}
	if mode != "ids" && mode != "filter" {
		return pgvector.Failf(
			`%q isn't a way of choosing what to delete — pick "Documents with these IDs" or `+
				`"Documents matching a metadata filter"`, mode)
	}

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

	relation, err := pgvector.QuoteRelation(auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	collection := pgvector.GetCollection(inputs)
	scope, err := collection.ResolveForRead(ctx, db, auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if collection.Active() && !scope.Exists {
		return pgvector.OK(map[string]interface{}{
			"deleted": 0,
			"result":  map[string]interface{}{"deleted": 0, "schema": auth.Schema, "table": auth.Table, "collection": collection.Name},
		}, fmt.Sprintf("There's no collection named %q in %s.%s, so nothing was deleted.",
			collection.Name, auth.Schema, auth.Table)), nil
	}

	var (
		whereClauses []string
		args         []interface{}
		ids          []string
	)
	// A collection scopes the whole delete — an ID or filter can only ever
	// reach rows inside it.
	if scope.Exists {
		clause, err := scope.ReadClause(&args)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		whereClauses = append(whereClauses, clause)
	}

	switch mode {
	case "ids":
		ids = parseIDs(pgvector.OptionalString(core.FindConnection("ids", inputs)))
		if len(ids) == 0 {
			return pgvector.Failf(
				`There are no Document IDs to delete. Add at least one ID, or switch Delete to ` +
					`"Documents matching a metadata filter"`)
		}
		if len(ids) > pgvector.MaxBatchDocuments {
			return pgvector.Failf(
				"That's %d IDs, and this step deletes at most %d at a time — split them across a Loop",
				len(ids), pgvector.MaxBatchDocuments)
		}
		// The ID column is as likely to be uuid or bigint as text, and casting the
		// column (rather than each value) makes one statement work for all three
		// while every ID stays a bound parameter. It costs the index on a
		// delete-by-ID, which is a fair trade for a list this small.
		args = append(args, pq.Array(ids))
		whereClauses = append(whereClauses, cols.QID+"::text = ANY($"+strconv.Itoa(len(args))+"::text[])")

	case "filter":
		filter, err := pgvector.BuildFilter(
			cols,
			[]*core.Connection{core.FindConnection("metadata_filter", inputs)},
			pgvector.OptionalString(core.FindConnection("metadata_filter_json", inputs)),
			len(args),
		)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		if filter.SQL == "" {
			// An empty filter with a collection means "delete this whole
			// collection", which is a deliberate scoped operation. An empty
			// filter with no collection would be DELETE with no WHERE — a table
			// wipe — and that is refused.
			if !scope.Exists {
				return pgvector.Failf(
					"That filter doesn't match on anything, which would delete every document in the table. " +
						"Add at least one condition, or set a Collection to clear just that collection.")
			}
		} else {
			whereClauses = append(whereClauses, filter.SQL)
			args = append(args, filter.Args...)
		}
	}

	if len(whereClauses) == 0 {
		return pgvector.Failf("There's nothing selected to delete — set some IDs, a filter, or a collection.")
	}
	where := strings.Join(whereClauses, " AND ")

	// RETURNING gives the real count and the IDs that actually went, which is what
	// a downstream step needs to log or reverse the change — RowsAffected would
	// give the count alone.
	rows, err := db.QueryContext(ctx,
		"DELETE FROM "+relation+" WHERE "+where+" RETURNING "+cols.QID+"::text", args...)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	defer rows.Close()

	deletedIDs := []string{}
	count := 0
	for rows.Next() {
		var id sql.NullString
		if err := rows.Scan(&id); err != nil {
			return pgvector.Fail(auth, err)
		}
		count++
		// The rows are already gone by the time we read them, so the cap bounds the
		// payload, not the delete.
		if len(deletedIDs) < pgvector.MaxRows {
			deletedIDs = append(deletedIDs, id.String)
		}
	}
	if err := rows.Err(); err != nil {
		return pgvector.Fail(auth, err)
	}

	table := auth.Schema + "." + auth.Table
	result := map[string]interface{}{
		"table":         table,
		"delete_by":     mode,
		"deleted":       count,
		"ids":           deletedIDs,
		"ids_truncated": count > len(deletedIDs),
	}

	summary := fmt.Sprintf("Deleted %d document%s from %s", count, plural(count), table)
	if count == 0 {
		summary = fmt.Sprintf("Nothing matched in %s, so no documents were deleted", table)
	}

	return pgvector.OK(map[string]interface{}{
		"deleted": count,
		"result":  result,
	}, summary), nil
}

// parseIDs reads the two shapes an ID list actually arrives in: the JSON array a
// previous step emits, and the comma-separated line a person types.
func parseIDs(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "[]" || raw == "null" {
		return nil
	}

	if strings.HasPrefix(raw, "[") {
		var arr []interface{}
		// UseNumber, not a plain Unmarshal: a bigint id column keyed by a
		// Snowflake-style value (~1.9e18) exceeds float64's 2^53 integer range,
		// so decoding the JSON number to float64 would silently round it and
		// target — or delete — the wrong row. json.Number keeps the exact text,
		// and idString passes it straight through.
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&arr); err == nil {
			out := make([]string, 0, len(arr))
			for _, v := range arr {
				if s := idString(v); s != "" {
					out = append(out, s)
				}
			}
			return out
		}
		// Malformed JSON, but the brackets tell us what was meant — fall through and
		// read it as a list rather than sending the operator away over punctuation.
		raw = strings.Trim(raw, "[]")
	}

	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.Trim(strings.TrimSpace(part), `"'`)
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// idString renders a JSON scalar as the text the ::text cast will compare
// against. A JSON number decodes to float64, and fmt would render 42 as "42" but
// 1e21 as "1e+21"; 'f' with precision -1 matches Postgres's own text output.
func idString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	default:
		return ""
	}
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
