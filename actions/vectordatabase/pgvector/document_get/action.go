package vectordatabase_pgvector_document_get

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Document"
	Description  = "Fetch a single document by its ID"
	Website      = "https://www.flomation.co"
	Icon         = "database+file"
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
	{Name: "id_column", Type: core.ConnectionTypeString, Label: "ID Column", Placeholder: "Leave empty to work it out — the primary key, or a column called \"id\""},
	{Name: "content_column", Type: core.ConnectionTypeString, Label: "Content Column", Placeholder: "Leave empty to work it out — text, content, document…"},
	{Name: "metadata_column", Type: core.ConnectionTypeString, Label: "Metadata Column", Placeholder: "Leave empty to work it out — metadata, meta…"},
	{Name: "vector_column", Type: core.ConnectionTypeString, Label: "Embedding Column", Placeholder: "Leave empty to work it out — the table's vector column"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Document ID", Placeholder: "The ID of the document to fetch", Required: true},
	{Name: "include_vectors", Type: core.ConnectionTypeBoolean, Label: "Include the raw embedding", Placeholder: "Off by default — an embedding is thousands of numbers and is rarely wanted downstream"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Document ID"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Content"},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Metadata"},
	{Name: "embedding", Type: core.ConnectionTypeObject, Label: "Embedding"},
	{Name: "found", Type: core.ConnectionTypeBoolean, Label: "Found"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Document"},
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
		return pgvector.Failf("Table is required — pick the table the document lives in")
	}

	docID := pgvector.OptionalString(core.FindConnection("id", inputs))
	if docID == "" {
		return pgvector.Failf("Document ID is required — this step fetches one document, by its ID")
	}
	includeVectors := pgvector.OptionalBool(core.FindConnection("include_vectors", inputs), false)

	relation, err := pgvector.QuoteRelation(auth.Schema, auth.Table)
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

	cols, err := pgvector.ResolveColumns(ctx, db, auth.Schema, auth.Table, pgvector.ColumnInputs{
		ID:       pgvector.OptionalString(core.FindConnection("id_column", inputs)),
		Content:  pgvector.OptionalString(core.FindConnection("content_column", inputs)),
		Metadata: pgvector.OptionalString(core.FindConnection("metadata_column", inputs)),
		Vector:   pgvector.OptionalString(core.FindConnection("vector_column", inputs)),
	})
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	// The embedding is only fetched when it is asked for: it is the largest
	// column in the table by a wide margin, and almost nothing downstream of a
	// "fetch this document" step wants to read 1536 floats.
	selected := []string{cols.QID, cols.QContent}
	if cols.HasMetadata() {
		selected = append(selected, cols.QMetadata)
	}
	if includeVectors {
		selected = append(selected, cols.QVector)
	}

	// Casting the ID column to text is what lets one string input address a uuid,
	// a bigint and a text primary key without the operator ever being asked which
	// they have. It does forgo the index on that column, which on a very large
	// table shows up as a slow lookup rather than a wrong answer — an acceptable
	// trade for a single-document fetch.
	query := "SELECT " + strings.Join(selected, ", ") +
		" FROM " + relation +
		" WHERE " + cols.QID + "::text = $1 LIMIT 1"

	var (
		id      sql.NullString
		content sql.NullString
		metaRaw []byte
		vecRaw  interface{}
	)
	dest := []interface{}{&id, &content}
	if cols.HasMetadata() {
		dest = append(dest, &metaRaw)
	}
	if includeVectors {
		dest = append(dest, &vecRaw)
	}

	if err := db.QueryRowContext(ctx, query, docID).Scan(dest...); err != nil {
		// A missing document is an answer, not a failure. A flow that branches on
		// "have we already stored this?" must be able to read found=false off the
		// success port rather than being routed down the error port.
		if errors.Is(err, sql.ErrNoRows) {
			return pgvector.OK(map[string]interface{}{
				"id":        docID,
				"content":   "",
				"metadata":  map[string]interface{}{},
				"embedding": []float32{},
				"found":     false,
				"result":    map[string]interface{}{},
			}, fmt.Sprintf("No document with ID %q in %s.%s", docID, auth.Schema, auth.Table)), nil
		}
		return pgvector.Fail(auth, err)
	}

	// jsonb comes back as raw bytes, so it has to be decoded here or every
	// downstream step would be handed a base64 blob instead of a metadata object.
	metadata := map[string]interface{}{}
	if len(metaRaw) > 0 {
		if err := json.Unmarshal(metaRaw, &metadata); err != nil {
			return pgvector.Failf(
				"the %s column on document %q holds something that isn't a JSON object, so its metadata can't be read",
				cols.Metadata, docID)
		}
	}

	embedding := []float32{}
	if includeVectors {
		vec, err := pgvector.ParseVector(vecRaw)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		// A row whose embedding is NULL decodes to nil; keep the output an empty
		// list either way so a downstream step never has to type-check it.
		if vec != nil {
			embedding = vec
		}
	}

	document := map[string]interface{}{
		"id":       id.String,
		"content":  content.String,
		"metadata": metadata,
	}
	if includeVectors {
		document["embedding"] = embedding
	}

	summary := fmt.Sprintf("Found document %q in %s.%s", id.String, auth.Schema, auth.Table)
	if preview := pgvector.Preview(content.String); preview != "" {
		summary += ": " + preview
	}

	return pgvector.OK(map[string]interface{}{
		"id":        id.String,
		"content":   content.String,
		"metadata":  metadata,
		"embedding": embedding,
		"found":     true,
		"result":    document,
	}, summary), nil
}
