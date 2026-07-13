package vectordatabase_pgvector_document_update

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	ai_common "flomation.app/automate/executor/actions/ai"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
	"github.com/lib/pq"
)

// Editing a stored document is the one thing the n8n PGVector node cannot do at
// all: LangChain's PGVectorStore exposes add and delete and nothing in between,
// so n8n's node throws on any update. The workaround everyone reaches for —
// delete the row and insert it again — loses the row's identity and anything
// else the table was carrying on it.
//
// The point of doing it properly is that text and embedding must not be allowed
// to drift apart. A row whose content says one thing and whose vector was
// computed from another is invisible to search and silently wrong, so changing
// the text re-embeds it in the same statement by default.

const (
	Author       = "Ethan Tan"
	Organisation = "Flomation"
	Name         = "Update Document"
	Description  = "Change a document's text or metadata, re-embedding it automatically"
	Website      = "https://www.flomation.co"
	Icon         = "database+pen"
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
	{Name: "id_column", Type: core.ConnectionTypeString, Label: "ID Column", Placeholder: "Leave empty to work it out automatically"},
	{Name: "content_column", Type: core.ConnectionTypeString, Label: "Content Column", Placeholder: "Leave empty to work it out automatically"},
	{Name: "metadata_column", Type: core.ConnectionTypeString, Label: "Metadata Column", Placeholder: "Leave empty to work it out automatically"},
	{Name: "vector_column", Type: core.ConnectionTypeString, Label: "Embedding Column", Placeholder: "Leave empty to work it out automatically"},
	{Name: "embedding_source", Type: core.ConnectionTypeString, Label: "Embedding Source", Required: true, Options: pgvector.EmbedSourceOptions},
	{Name: "embedding", Type: core.ConnectionTypeObject, Label: "Embedding Vector", Placeholder: "Pick the Embedding output of an Embed Text step", Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"vector"}}},
	{Name: "provider", Type: core.ConnectionTypeString, Label: "Embedding Provider", Required: true, Options: ai_common.EmbedProviderOptions, Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Embedding API Key", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"openai", "openai_compatible"}}},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Embedding Base URL", Placeholder: "http://ollama.internal:11434", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"openai_compatible", "ollama"}}},
	{Name: "access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key ID", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Access Key", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "AWS Region", Placeholder: "us-east-1", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "model", Type: core.ConnectionTypeString, Label: "Embedding Model", Placeholder: "text-embedding-3-small", Options: ai_common.EmbedModelOptions, Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
	{Name: "dimensions", Type: core.ConnectionTypeInteger, Label: "Dimensions", Placeholder: "Leave empty for the model's default — must match the table", Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Document ID", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "New Content", Placeholder: "Leave empty to keep the current text"},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `{"reviewed": true}`},
	{Name: "metadata_merge", Type: core.ConnectionTypeBoolean, Label: "Merge into the existing metadata rather than replacing it", Value: true},
	{Name: "reembed", Type: core.ConnectionTypeBoolean, Label: "Re-generate the embedding from the new text", Value: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Document ID"},
	{Name: "updated", Type: core.ConnectionTypeBoolean, Label: "Updated"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Document"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := pgvector.GetAuth(inputs)
	if err != nil {
		return pgvector.ErrorResult(err.Error()), nil
	}
	if auth.Table == "" {
		return pgvector.Failf("Table is required — pick the table the document lives in")
	}

	id := pgvector.OptionalString(core.FindConnection("id", inputs))
	if id == "" {
		return pgvector.Failf("Document ID is required — this step needs to know which document to change")
	}
	content := pgvector.OptionalString(core.FindConnection("content", inputs))
	metaJSON, hasMeta, err := readMetadata(core.FindConnection("metadata", inputs))
	if err != nil {
		return pgvector.ErrorResult(err.Error()), nil
	}
	merge := pgvector.OptionalBool(core.FindConnection("metadata_merge", inputs), true)
	reembed := pgvector.OptionalBool(core.FindConnection("reembed", inputs), true)

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

	label := auth.Schema + "." + auth.Table
	if hasMeta && !cols.HasMetadata() {
		return pgvector.Failf(
			"%s has no metadata column, so there's nowhere to put the Metadata — leave it empty, or point this "+
				"step at a table with a jsonb metadata column", label)
	}

	// Two routes to a new vector, and they are gated differently. A vector wired
	// in from a previous step is an explicit instruction to store that vector, so
	// it is always written. Re-embedding, on the other hand, only has anything to
	// work from when new text was supplied: an operator updating a document's
	// metadata alone has left the text — and therefore the embedding — untouched,
	// and re-embedding on a blank field would be an error message about nothing.
	source := pgvector.OptionalString(core.FindConnection("embedding_source", inputs))
	if source == "" {
		source = "inline"
	}
	var vec []float32
	if source == "vector" || (reembed && content != "") {
		spec, err := pgvector.GetEmbedSpec(inputs)
		if err != nil {
			return pgvector.ErrorResult(err.Error()), nil
		}
		vec, err = spec.EmbedOne(ctx, content)
		if err != nil {
			return pgvector.Failf("Couldn't create the embedding: %s", spec.EmbedError(err))
		}

		declared, err := pgvector.TableDimension(ctx, db, auth.Schema, auth.Table, cols.Vector)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		if err := pgvector.CheckDimension(declared, vec, label); err != nil {
			return pgvector.ErrorResult(err.Error()), nil
		}
	}

	// The SET list is whatever the operator actually filled in — an update that
	// touches only the metadata must not overwrite the text with an empty string.
	var (
		sets    []string
		args    []interface{}
		changed []string
	)
	arg := func(v interface{}) string {
		args = append(args, v)
		return fmt.Sprintf("$%d", len(args))
	}

	if content != "" {
		sets = append(sets, cols.QContent+" = "+arg(content))
		changed = append(changed, "new text")
	}
	if hasMeta {
		if merge {
			// COALESCE because jsonb's || is NULL-propagating: merging into a row
			// whose metadata was never set would otherwise erase the update.
			sets = append(sets, fmt.Sprintf("%s = COALESCE(%s, '{}'::jsonb) || %s::jsonb",
				cols.QMetadata, cols.QMetadata, arg(metaJSON)))
			changed = append(changed, "metadata merged in")
		} else {
			sets = append(sets, cols.QMetadata+" = "+arg(metaJSON)+"::jsonb")
			changed = append(changed, "metadata replaced")
		}
	}
	if len(vec) > 0 {
		sets = append(sets, cols.QVector+" = "+arg(pgvector.VectorLiteral(vec))+"::vector")
		if source == "vector" {
			changed = append(changed, "embedding replaced")
		} else {
			changed = append(changed, "embedding re-generated")
		}
	}
	if len(sets) == 0 {
		return pgvector.Failf("There's nothing to update — set New Content or Metadata")
	}

	qrel, err := pgvector.QuoteRelation(auth.Schema, auth.Table)
	if err != nil {
		return pgvector.ErrorResult(err.Error()), nil
	}

	// The ID column may be uuid, text or bigint depending on who built the table,
	// and the flow has no way of knowing which. Binding the ID as text and letting
	// Postgres coerce it to the column's own type is what makes one action work
	// against all three.
	returning := []string{cols.QID, cols.QContent}
	if cols.HasMetadata() {
		returning = append(returning, cols.QMetadata)
	}
	query := "UPDATE " + qrel +
		" SET " + strings.Join(sets, ", ") +
		" WHERE " + cols.QID + " = " + arg(id) +
		" RETURNING " + strings.Join(returning, ", ")

	var (
		gotID      string
		gotContent sql.NullString
		gotMeta    []byte
	)
	dest := []interface{}{&gotID, &gotContent}
	if cols.HasMetadata() {
		dest = append(dest, &gotMeta)
	}

	switch err := db.QueryRowContext(ctx, query, args...).Scan(dest...); {
	case errors.Is(err, sql.ErrNoRows):
		// Not a failure. Updating a document that isn't there is a fact about the
		// table, and a flow that deletes then tidies up should not be routed down
		// its error branch for it.
		return pgvector.OK(map[string]interface{}{
			"id":      id,
			"updated": false,
			"result":  map[string]interface{}{},
		}, fmt.Sprintf("No document with ID %q in %s — nothing was changed.", id, label)), nil

	case err != nil:
		var pe *pq.Error
		if errors.As(err, &pe) && pe.Code == "22P02" { // invalid_text_representation
			if t := idColumnType(ctx, db, auth.Schema, auth.Table, cols.ID); t != "" {
				return pgvector.Failf("%q isn't a valid ID for this table — the ID column is a %s.", id, t)
			}
			return pgvector.Failf("%q isn't a valid ID for this table.", id)
		}
		return pgvector.Fail(auth, err)
	}

	result := map[string]interface{}{"id": gotID}
	if gotContent.Valid {
		result["content"] = gotContent.String
	}
	if len(gotMeta) > 0 {
		var meta interface{}
		if err := json.Unmarshal(gotMeta, &meta); err == nil {
			result["metadata"] = meta
		}
	}

	summary := fmt.Sprintf("Updated document %q in %s — %s", gotID, label, strings.Join(changed, ", "))
	if content != "" {
		summary += ". New text: " + pgvector.Preview(content)
	}
	return pgvector.OK(map[string]interface{}{
		"id":      gotID,
		"updated": true,
		"result":  result,
	}, summary), nil
}

// readMetadata reads the Metadata input.
//
// It cannot go through Connection.String(): an Object input holding a live
// map from an upstream step stringifies with Go's %v formatting —
// "map[reviewed:true]" — which is not JSON and would be rejected by jsonb. The
// value is taken raw instead, so a JSON string (what the editor stores, and what
// a ${...} reference resolves to) and a real map both work.
//
// An empty object counts as "not supplied", the same way filter.go treats an
// empty key/value grid. That costs the ability to blank a document's metadata by
// replacing it with {}, and buys the guarantee that an untouched field — which
// arrives as "{}" — can never silently wipe metadata off a row whose text was
// the only thing being changed.
func readMetadata(c *core.Connection) (string, bool, error) {
	if c == nil || c.Value == nil {
		return "", false, nil
	}

	var raw []byte
	switch v := c.Value.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" || s == "{}" || s == "null" {
			return "", false, nil
		}
		raw = []byte(s)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", false, fmt.Errorf("couldn't read the Metadata: %v", err)
		}
		raw = b
	}

	var obj map[string]interface{}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", false, fmt.Errorf(
			`the Metadata isn't a valid JSON object: %v. It should look like {"reviewed": true}`, err)
	}
	if len(obj) == 0 {
		return "", false, nil
	}

	// Re-encoded so that whatever shape it arrived in, one canonical JSON object
	// reaches the database.
	out, err := json.Marshal(obj)
	if err != nil {
		return "", false, fmt.Errorf("couldn't read the Metadata: %v", err)
	}
	return string(out), true, nil
}

// idColumnType reads the declared type of the ID column, so that an ID Postgres
// refused to coerce can be explained in terms of what the table actually wants
// rather than as an SQLSTATE. Best-effort: if the catalog lookup fails the
// caller falls back to the plainer message.
func idColumnType(ctx context.Context, db *sql.DB, schema, table, column string) string {
	var t string
	err := db.QueryRowContext(ctx, `
		SELECT format_type(a.atttypid, a.atttypmod)
		  FROM pg_attribute a
		  JOIN pg_class     c ON c.oid = a.attrelid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = $1
		   AND c.relname = $2
		   AND a.attname = $3
		   AND a.attnum  > 0
		   AND NOT a.attisdropped`, schema, table, column).Scan(&t)
	if err != nil {
		return ""
	}
	return t
}
