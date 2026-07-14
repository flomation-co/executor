// Upsert Documents — write documents, overwriting any that are already there.
//
// This action exists because LangChain (and therefore n8n) has no ON CONFLICT
// clause at all: writing a document whose ID is already in the table violates
// the primary key and the whole run dies. Re-ingesting a handbook that changed
// one paragraph means deleting the old rows first and hoping nothing else reads
// the table in between. An upsert is the operation people actually want when
// they re-import a source, so it gets its own step.
//
// Two things have to be true for it to work, and both are checked here rather
// than left to a Postgres error the operator cannot read:
//
//   - Postgres can only match a conflicting row against a UNIQUE index. Matching
//     by ID needs every document to carry one; matching by a metadata field
//     needs a unique index on that expression, which is not something a table
//     has by default.
//
//   - Chunking multiplies one document into several rows. That is fine when the
//     match is by ID (each chunk gets its own suffixed ID) and impossible when
//     the match is by a metadata field, because every chunk would carry the same
//     field value and collide with its own siblings.
package vectordatabase_pgvector_document_upsert

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
	"github.com/lib/pq"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Upsert Documents"
	Description  = "Insert documents, or overwrite them if they already exist"
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
	{Name: "ssl_mode", Type: core.ConnectionTypeString, Label: "SSL Mode", Placeholder: "disable", Options: []core.ConnectionOption{{Name: "Disable — no encryption", Value: "disable"}, {Name: "Allow", Value: "allow"}, {Name: "Prefer — encrypt if the server offers it", Value: "prefer"}, {Name: "Require — encrypt, but don't verify the certificate", Value: "require"}, {Name: "Verify CA — encrypt and check the certificate authority", Value: "verify-ca"}, {Name: "Verify Full — encrypt and check the hostname too", Value: "verify-full"}}},
	{Name: "schema", Type: core.ConnectionTypeString, Label: "Schema", Placeholder: "public"},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table", Placeholder: "documents", Required: true},

	{Name: "id_column", Type: core.ConnectionTypeString, Label: "ID Column", Placeholder: "Leave empty to work it out from the table"},
	{Name: "content_column", Type: core.ConnectionTypeString, Label: "Content Column", Placeholder: "Leave empty to work it out from the table"},
	{Name: "metadata_column", Type: core.ConnectionTypeString, Label: "Metadata Column", Placeholder: "Leave empty to work it out from the table"},
	{Name: "vector_column", Type: core.ConnectionTypeString, Label: "Embedding Column", Placeholder: "Leave empty to work it out from the table"},

	{Name: "embedding_source", Type: core.ConnectionTypeString, Label: "Embedding Source", Required: true, Options: []core.ConnectionOption{{Name: "Embed the text for me", Value: "inline"}, {Name: "Use a vector from a previous step", Value: "vector"}}},
	{Name: "embedding", Type: core.ConnectionTypeObject, Label: "Embedding Vector", Placeholder: "Pick the Embedding output of an Embed Text step", Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"vector"}}},
	{Name: "provider", Type: core.ConnectionTypeString, Label: "Embedding Provider", Required: true, Options: []core.ConnectionOption{{Name: "OpenAI", Value: "openai"}, {Name: "OpenAI-compatible (Azure, vLLM, LocalAI, TEI…)", Value: "openai_compatible"}, {Name: "Ollama (self-hosted)", Value: "ollama"}, {Name: "AWS Bedrock (Titan)", Value: "bedrock"}}, Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Embedding API Key", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"openai", "openai_compatible"}}},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Embedding Base URL", Placeholder: "http://ollama.internal:11434", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"openai_compatible", "ollama"}}},
	{Name: "access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key ID", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Access Key", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "AWS Region", Placeholder: "us-east-1", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "model", Type: core.ConnectionTypeComboBox, Label: "Embedding Model", Placeholder: "text-embedding-3-small", Options: []core.ConnectionOption{{Name: "OpenAI text-embedding-3-small (1536 dimensions)", Value: "text-embedding-3-small"}, {Name: "OpenAI text-embedding-3-large (3072 dimensions)", Value: "text-embedding-3-large"}, {Name: "OpenAI text-embedding-ada-002 (1536 dimensions)", Value: "text-embedding-ada-002"}, {Name: "Bedrock Titan Text v2 (1024 dimensions)", Value: "amazon.titan-embed-text-v2:0"}, {Name: "Bedrock Titan Text v1 (1536 dimensions)", Value: "amazon.titan-embed-text-v1"}, {Name: "Ollama nomic-embed-text (768 dimensions)", Value: "nomic-embed-text"}, {Name: "Ollama mxbai-embed-large (1024 dimensions)", Value: "mxbai-embed-large"}}, Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
	{Name: "dimensions", Type: core.ConnectionTypeInteger, Label: "Dimensions", Placeholder: "Leave empty for the model's default — must match the table", Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},

	{Name: "content", Type: core.ConnectionTypeText, Label: "Content", Placeholder: "The document text to store"},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `{"source": "handbook", "page": 3}`},
	{Name: "id", Type: core.ConnectionTypeString, Label: "ID", Placeholder: "The document's ID — the same ID next time overwrites this document"},
	{Name: "documents", Type: core.ConnectionTypeObject, Label: "Documents (JSON list)", Placeholder: `[{"id": "doc-1", "content": "…", "metadata": {"source": "handbook"}}] — upsert many at once, instead of Content above`},
	{Name: "chunk_size", Type: core.ConnectionTypeInteger, Label: "Chunk Size", Placeholder: "Split long text into pieces of this many characters — leave empty to store it whole"},
	{Name: "chunk_overlap", Type: core.ConnectionTypeInteger, Label: "Chunk Overlap", Placeholder: "Characters each piece repeats from the one before, e.g. 200"},

	{
		Name:  "conflict_target",
		Type:  core.ConnectionTypeString,
		Label: "Match Existing Documents By",
		Options: []core.ConnectionOption{
			{Name: "ID", Value: "id"},
			{Name: "A metadata field", Value: "metadata_key"},
		},
	},
	{Name: "conflict_metadata_key", Type: core.ConnectionTypeString, Label: "Metadata Field", Placeholder: "external_id — needs a unique index on (metadata->>'external_id')", Visible: &core.VisibleWhen{Field: "conflict_target", Values: []string{"metadata_key"}}},
	{Name: "collection", Type: core.ConnectionTypeString, Label: "Collection", Placeholder: "Optional — tag these documents as part of a named collection within the table"},
	{Name: "collection_table", Type: core.ConnectionTypeString, Label: "Collection Table", Placeholder: "flomation_vector_collections — where collection names are recorded"},
}

var Outputs = [...]core.Connection{
	{Name: "ids", Type: core.ConnectionTypeObject, Label: "Document IDs"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Documents Written"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// metaKeyRe bounds the one value in this action that cannot be bound as a
// parameter. ON CONFLICT ((metadata->>'k')) has to match an index expression,
// which Postgres resolves at plan time, so 'k' must be in the SQL text before
// any parameter exists. Anything that is not a plain field name is refused
// outright rather than escaped and hoped for — a metadata key with a quote in
// it is not a real key, it is an attack.
var metaKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,62}$`)

// doc is one document as the operator supplied it, before chunking.
type doc struct {
	Index    int // 1-based, for error messages the operator can act on
	ID       string
	HasID    bool
	Content  string
	Meta     map[string]interface{}
	MetaJSON string
	HasMeta  bool
	Vector   []float32 // supplied per-document, skipping the embedding call
}

// row is one database row: a document, or one chunk of one.
type row struct {
	DocIndex int
	ID       string
	HasID    bool
	Content  string
	MetaJSON string
	Vector   []float32
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := pgvector.GetAuth(inputs)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if auth.Table == "" {
		return pgvector.Failf("Table is required — name the table to write the documents into")
	}
	qrel, err := pgvector.QuoteRelation(auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	label := auth.Schema + "." + auth.Table

	embedSpec, err := pgvector.GetEmbedSpec(inputs)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	matchBy := pgvector.OptionalString(core.FindConnection("conflict_target", inputs))
	if matchBy == "" {
		matchBy = "id"
	}
	metaKey := ""
	switch matchBy {
	case "id":
	case "metadata_key":
		metaKey = pgvector.OptionalString(core.FindConnection("conflict_metadata_key", inputs))
		if metaKey == "" {
			return pgvector.Failf(
				"Match Existing Documents By is set to a metadata field, but no Metadata Field was named — " +
					"say which field identifies a document, e.g. external_id")
		}
		if !metaKeyRe.MatchString(metaKey) {
			return pgvector.Failf(
				"%q isn't a usable metadata field name — use letters, numbers and underscores, starting with a letter",
				metaKey)
		}
	default:
		return pgvector.Failf(
			"%q isn't a valid choice for Match Existing Documents By — pick ID or a metadata field", matchBy)
	}

	docs, err := collectDocuments(inputs)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if len(docs) > pgvector.MaxBatchDocuments {
		return pgvector.Failf(
			"that's %d documents, and one Upsert step can write at most %d — split the list into smaller batches",
			len(docs), pgvector.MaxBatchDocuments)
	}

	chunkSize := pgvector.OptionalInt(core.FindConnection("chunk_size", inputs), 0)
	chunkOverlap := pgvector.OptionalInt(core.FindConnection("chunk_overlap", inputs), 0)
	if chunkSize > 0 && chunkOverlap >= chunkSize {
		return pgvector.Failf(
			"Chunk Overlap (%d) has to be smaller than Chunk Size (%d), or the pieces would never move forward",
			chunkOverlap, chunkSize)
	}

	rows, suffixed, err := buildRows(docs, chunkSize, chunkOverlap, matchBy, metaKey)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if len(rows) > pgvector.MaxBatchDocuments {
		return pgvector.Failf(
			"chunking those %d documents produces %d rows, and one Upsert step can write at most %d — "+
				"use a bigger Chunk Size, or upsert fewer documents at a time",
			len(docs), len(rows), pgvector.MaxBatchDocuments)
	}
	if err := checkDuplicateKeys(rows, metaKey); err != nil {
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
	if !cols.HasMetadata() {
		if metaKey != "" {
			return pgvector.Failf(
				"%s has no metadata column, so there's no metadata field to match documents on — "+
					"switch Match Existing Documents By to ID", label)
		}
		// Writing the documents anyway would quietly drop the metadata the
		// operator took the trouble to supply.
		if anyMeta(rows) {
			return pgvector.Failf(
				"%s has no metadata column, so there's nowhere to put the metadata on these documents — "+
					"remove it, or point this step at a table with a jsonb metadata column", label)
		}
	}

	// Embedding is the only step here that costs money, so it runs after the
	// table and the columns have been proved good.
	if err := embedRows(ctx, embedSpec, rows); err != nil {
		return pgvector.Failf("%s", embedSpec.EmbedError(err))
	}

	declared, err := pgvector.TableDimension(ctx, db, auth.Schema, auth.Table, cols.Vector)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	for _, r := range rows {
		if len(r.Vector) == 0 {
			return pgvector.Failf("document %d ended up with no embedding to store", r.DocIndex)
		}
		if err := pgvector.CheckDimension(declared, r.Vector, label); err != nil {
			return pgvector.Fail(auth, err)
		}
	}

	// The metadata column is left out of the statement entirely when no document
	// carries any: an upsert that overwrote existing metadata with {} simply
	// because this step had none to offer would be data loss.
	includeMeta := cols.HasMetadata() && (metaKey != "" || anyMeta(rows))
	includeID := anyID(rows)

	// A collection stamps every written row with its id, provisioning the
	// collection table and the collection_id column the first time.
	collection := pgvector.GetCollection(inputs)
	collectionID := ""
	if collection.Active() {
		collectionID, err = collection.ResolveForWrite(ctx, db, auth.Schema, auth.Table)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
	}

	stmt, args := buildStatement(qrel, cols, rows, includeID, includeMeta, conflictTarget(cols, metaKey), collectionID)

	result, err := db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return pgvector.Failf("%s", upsertError(auth, err, label, matchBy, metaKey, cols, suffixed))
	}
	defer result.Close()

	ids := make([]interface{}, 0, len(rows))
	for result.Next() {
		var raw interface{}
		if err := result.Scan(&raw); err != nil {
			return pgvector.Fail(auth, err)
		}
		ids = append(ids, normaliseID(raw))
	}
	if err := result.Err(); err != nil {
		return pgvector.Fail(auth, err)
	}

	matched := "ID"
	if metaKey != "" {
		matched = "metadata field \"" + metaKey + "\""
	}
	return pgvector.OK(map[string]interface{}{
		"ids":   ids,
		"count": len(ids),
		"result": map[string]interface{}{
			"table":      label,
			"documents":  len(docs),
			"rows":       len(ids),
			"matched_by": matchBy,
			"ids":        ids,
		},
	}, summary(docs, rows, ids, label, matched)), nil
}

// ---------------------------------------------------------------------------
// Input
// ---------------------------------------------------------------------------

// collectDocuments reads either the Documents list or the single-document
// fields. The list wins when it has anything in it, so a flow that switches
// from one document to many does not silently write both.
func collectDocuments(inputs []*core.Connection) ([]doc, error) {
	if c := core.FindConnection("documents", inputs); c != nil && c.Value != nil {
		docs, err := parseDocuments(c.Value)
		if err != nil {
			return nil, err
		}
		if len(docs) > 0 {
			return docs, nil
		}
	}

	content := pgvector.OptionalString(core.FindConnection("content", inputs))
	if content == "" {
		return nil, errors.New(
			"there's nothing to upsert — put the document text in Content, or a list of documents in Documents")
	}

	d := doc{Index: 1, Content: content}
	if id := pgvector.OptionalString(core.FindConnection("id", inputs)); id != "" {
		d.ID, d.HasID = id, true
	}
	if c := core.FindConnection("metadata", inputs); c != nil && c.Value != nil {
		meta, ok, err := parseMetadata(c.Value, 1)
		if err != nil {
			return nil, err
		}
		if ok {
			d.Meta, d.HasMeta = meta, true
		}
	}
	if err := d.encodeMeta(); err != nil {
		return nil, err
	}
	return []doc{d}, nil
}

// rawDoc is one entry of the Documents list. "text" is accepted alongside
// "content" because that is the column LangChain writes, so a list exported
// from an existing corpus works without being rewritten.
type rawDoc struct {
	Content   interface{} `json:"content"`
	Text      interface{} `json:"text"`
	ID        interface{} `json:"id"`
	Metadata  interface{} `json:"metadata"`
	Embedding interface{} `json:"embedding"`
}

func parseDocuments(val interface{}) ([]doc, error) {
	var data []byte
	switch v := val.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" || s == "null" || s == "[]" || s == "{}" {
			return nil, nil
		}
		data = []byte(s)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, errors.New("couldn't read the Documents list")
		}
		data = b
	}

	// UseNumber so a bigint id (a Snowflake-style ~1.9e18 value) is not rounded
	// through float64 into a different id — which for an upsert would overwrite
	// the wrong row. scalarString passes json.Number straight through.
	decode := func(into interface{}) error {
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.UseNumber()
		return dec.Decode(into)
	}

	var list []rawDoc
	if err := decode(&list); err != nil {
		// A single object is a reasonable thing to wire in, so accept it too.
		var single rawDoc
		if err2 := decode(&single); err2 != nil {
			return nil, fmt.Errorf(
				"couldn't read the Documents list — it should look like "+
					`[{"id": "doc-1", "content": "…", "metadata": {"source": "handbook"}}] (%v)`, err)
		}
		list = []rawDoc{single}
	}

	docs := make([]doc, 0, len(list))
	for i, r := range list {
		n := i + 1
		content, err := textOf(r.Content, r.Text, n)
		if err != nil {
			return nil, err
		}
		if content == "" {
			return nil, fmt.Errorf("document %d has no content — every document needs some text to store", n)
		}

		d := doc{Index: n, Content: content}
		if id := scalarString(r.ID); id != "" {
			d.ID, d.HasID = id, true
		}
		if r.Metadata != nil {
			meta, ok, err := parseMetadata(r.Metadata, n)
			if err != nil {
				return nil, err
			}
			if ok {
				d.Meta, d.HasMeta = meta, true
			}
		}
		if r.Embedding != nil {
			vec, err := pgvector.CoerceVector(r.Embedding)
			if err != nil {
				return nil, fmt.Errorf("document %d's embedding: %v", n, err)
			}
			d.Vector = vec
		}
		if err := d.encodeMeta(); err != nil {
			return nil, err
		}
		docs = append(docs, d)
	}
	return docs, nil
}

// encodeMeta renders the metadata once, so the same JSON text is bound for
// every chunk of the document rather than re-marshalled per row.
func (d *doc) encodeMeta() error {
	if !d.HasMeta {
		d.MetaJSON = "{}"
		return nil
	}
	b, err := json.Marshal(d.Meta)
	if err != nil {
		return fmt.Errorf("couldn't read the metadata for document %d", d.Index)
	}
	d.MetaJSON = string(b)
	return nil
}

func parseMetadata(val interface{}, n int) (map[string]interface{}, bool, error) {
	var data []byte
	switch v := val.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" || s == "null" || s == "{}" {
			return nil, false, nil
		}
		data = []byte(s)
	case map[string]interface{}:
		if len(v) == 0 {
			return nil, false, nil
		}
		return v, true, nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, false, metaShapeError(n)
		}
		data = b
	}

	var meta map[string]interface{}
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil, false, metaShapeError(n)
	}
	if len(meta) == 0 {
		return nil, false, nil
	}
	return meta, true, nil
}

func metaShapeError(n int) error {
	return fmt.Errorf(
		`the metadata for document %d isn't a JSON object — it should look like {"source": "handbook", "page": 3}`, n)
}

// textOf reads a document's text from whichever of the two field names it used.
func textOf(content, text interface{}, n int) (string, error) {
	for _, v := range []interface{}{content, text} {
		switch t := v.(type) {
		case nil:
			continue
		case string:
			if s := strings.TrimSpace(t); s != "" {
				return s, nil
			}
		default:
			return "", fmt.Errorf("document %d's content has to be text, not a %T", n, v)
		}
	}
	return "", nil
}

// scalarString renders an ID the way Postgres will read it back. An ID that
// arrived as a JSON number (a bigint primary key) is bound as its text form,
// which the server parses back into the column's own type.
func scalarString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case json.Number:
		return t.String()
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", t))
	}
}

// ---------------------------------------------------------------------------
// Rows
// ---------------------------------------------------------------------------

// buildRows chunks each document and reports whether any chunk ID was suffixed.
func buildRows(docs []doc, size, overlap int, matchBy, metaKey string) ([]row, bool, error) {
	var rows []row
	suffixed := false

	for _, d := range docs {
		if matchBy == "id" && !d.HasID {
			return nil, false, errors.New(
				"Upserting by ID needs each document to have an ID — set the ID field, or switch " +
					"Match Existing Documents By to a metadata field")
		}
		if metaKey != "" {
			if v, ok := d.Meta[metaKey]; !ok || scalarString(v) == "" {
				return nil, false, fmt.Errorf(
					"document %d has no %q in its metadata, so there's nothing to match it on — give every document "+
						"that field, or switch Match Existing Documents By to ID", d.Index, metaKey)
			}
		}

		chunks := chunkText(d.Content, size, overlap, pgvector.MaxBatchDocuments+1)
		if len(chunks) > 1 {
			// Every chunk of a document carries the same metadata, so every chunk
			// would claim the same conflict key — they would fight each other for
			// the one row that key is allowed to occupy.
			if metaKey != "" {
				return nil, false, fmt.Errorf(
					"Chunk Size splits document %d into %d pieces, but matching on the metadata field %q allows only "+
						"one row per document — turn Chunk Size off, or switch Match Existing Documents By to ID",
					d.Index, len(chunks), metaKey)
			}
			if len(d.Vector) > 0 {
				return nil, false, fmt.Errorf(
					"document %d supplies its own embedding, but Chunk Size splits it into %d pieces and each piece "+
						"needs an embedding of its own — turn Chunk Size off, or let this step embed the text",
					d.Index, len(chunks))
			}
		}

		for i, text := range chunks {
			r := row{
				DocIndex: d.Index,
				Content:  text,
				MetaJSON: d.MetaJSON,
				Vector:   d.Vector,
			}
			if d.HasID {
				r.ID, r.HasID = d.ID, true
				// Chunks of one document need one ID each, and those IDs have to be
				// stable across runs or a re-upsert would append duplicates instead
				// of overwriting. Deriving them from the document's own ID gives
				// both. Note that shrinking a document leaves its surplus chunks
				// behind — delete by metadata first if that matters.
				if len(chunks) > 1 {
					r.ID = d.ID + "#" + strconv.Itoa(i+1)
					suffixed = true
				}
			}
			rows = append(rows, r)
		}
	}
	return rows, suffixed, nil
}

// chunkText splits text into overlapping windows, measured in characters.
//
// It counts runes rather than bytes so that a window boundary can never land
// inside a multi-byte character, and it prefers to break at whitespace near the
// end of a window so a chunk does not begin mid-word — an embedding of "…the
// compa" is a worse representation of the text than an embedding of "…the".
func chunkText(s string, size, overlap, limit int) []string {
	if size <= 0 {
		return []string{s}
	}
	runes := []rune(s)
	if len(runes) <= size {
		return []string{s}
	}
	if overlap < 0 {
		overlap = 0
	}
	// An overlap at or above the chunk size steps the window backwards, so the
	// only thing stopping a runaway is the per-iteration guard below — which
	// caps the STEP at one rune, not the count. A 2 MB document then yields
	// millions of chunks and exhausts memory before the batch cap is ever
	// checked. Bound the overlap, and bound the loop by `limit` as insert does.
	if overlap >= size {
		overlap = size / 2
	}

	var out []string
	for start := 0; start < len(runes) && len(out) < limit; {
		end := start + size
		if end >= len(runes) {
			out = append(out, strings.TrimSpace(string(runes[start:])))
			break
		}

		cut := end
		for i := end; i > end-size/5 && i > start; i-- {
			if isSpace(runes[i]) {
				cut = i
				break
			}
		}

		out = append(out, strings.TrimSpace(string(runes[start:cut])))

		next := cut - overlap
		if next <= start {
			// A large overlap combined with a backtracked cut can point the next
			// window at or behind this one. Give up the overlap rather than loop
			// forever.
			next = cut
		}
		start = next
	}

	// TrimSpace can empty a window that was pure whitespace.
	kept := out[:0]
	for _, c := range out {
		if c != "" {
			kept = append(kept, c)
		}
	}
	if len(kept) == 0 {
		return []string{s}
	}
	return kept
}

func isSpace(r rune) bool {
	return r == ' ' || r == '\n' || r == '\t' || r == '\r'
}

// checkDuplicateKeys catches two rows in the same batch claiming the same
// conflict key. Postgres refuses that outright ("cannot affect row a second
// time") because the second row would be updating a row the same statement has
// not committed yet, and its error names neither document.
func checkDuplicateKeys(rows []row, metaKey string) error {
	seen := make(map[string]int, len(rows))
	for _, r := range rows {
		key := r.ID
		what := "ID"
		if metaKey != "" {
			var meta map[string]interface{}
			if err := json.Unmarshal([]byte(r.MetaJSON), &meta); err != nil {
				continue
			}
			key = scalarString(meta[metaKey])
			what = metaKey
		}
		if key == "" {
			continue
		}
		if first, dup := seen[key]; dup && first != r.DocIndex {
			return fmt.Errorf(
				"documents %d and %d both have the %s %q — in one upsert each document has to have a different one, "+
					"or there's no telling which of them should win", first, r.DocIndex, what, key)
		}
		seen[key] = r.DocIndex
	}
	return nil
}

func anyID(rows []row) bool {
	for _, r := range rows {
		if r.HasID {
			return true
		}
	}
	return false
}

func anyMeta(rows []row) bool {
	for _, r := range rows {
		if r.MetaJSON != "" && r.MetaJSON != "{}" {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Embedding
// ---------------------------------------------------------------------------

// embedRows fills in every row that did not arrive with an embedding of its own,
// in one batched call.
func embedRows(ctx context.Context, spec pgvector.EmbedSpec, rows []row) error {
	var need []int
	var texts []string
	for i, r := range rows {
		if len(r.Vector) == 0 {
			need = append(need, i)
			texts = append(texts, r.Content)
		}
	}
	if len(need) == 0 {
		return nil
	}

	vecs, err := spec.EmbedTexts(ctx, texts)
	if err != nil {
		return err
	}
	if len(vecs) != len(need) {
		return fmt.Errorf("asked the embedding model for %d embeddings but got %d back", len(need), len(vecs))
	}
	for i, at := range need {
		rows[at].Vector = vecs[i]
	}
	return nil
}

// ---------------------------------------------------------------------------
// SQL
// ---------------------------------------------------------------------------

// conflictTarget renders what Postgres matches an existing row on.
//
// The metadata form is an expression, and it must be written exactly as the
// unique index that backs it was declared — ON CONFLICT does not search for a
// compatible index, it looks for an identical expression. The field name is the
// one thing in this file that reaches the SQL text as a literal; metaKeyRe has
// already refused anything that is not a plain field name, and the quote-doubling
// here is the second lock on a door that is already bolted.
// The caller wraps this in the one pair of parentheses ON CONFLICT's own syntax
// needs, so an expression target adds exactly one more of its own.
func conflictTarget(cols pgvector.ColumnSet, metaKey string) string {
	if metaKey == "" {
		return cols.QID
	}
	return "(" + cols.QMetadata + "->>'" + strings.ReplaceAll(metaKey, "'", "''") + "')"
}

func buildStatement(qrel string, cols pgvector.ColumnSet, rows []row, includeID, includeMeta bool, target, collectionID string) (string, []interface{}) {
	var b strings.Builder
	args := make([]interface{}, 0, len(rows)*5)
	withCollection := collectionID != ""

	names := make([]string, 0, 5)
	if includeID {
		names = append(names, cols.QID)
	}
	names = append(names, cols.QContent, cols.QVector)
	if includeMeta {
		names = append(names, cols.QMetadata)
	}
	if withCollection {
		names = append(names, pgvector.QuotedIDColumn())
	}

	b.WriteString("INSERT INTO " + qrel + " (" + strings.Join(names, ", ") + ") VALUES ")

	bind := func(v interface{}) string {
		args = append(args, v)
		return "$" + strconv.Itoa(len(args))
	}

	for i, r := range rows {
		if i > 0 {
			b.WriteString(", ")
		}
		vals := make([]string, 0, 5)
		if includeID {
			if r.HasID {
				vals = append(vals, bind(r.ID))
			} else {
				// A row in a mixed batch with no ID of its own: DEFAULT is the only
				// way to let the column's own default (a uuid or a sequence) fire,
				// because an explicit NULL would override it.
				vals = append(vals, "DEFAULT")
			}
		}
		vals = append(vals, bind(r.Content))
		vals = append(vals, bind(pgvector.VectorLiteral(r.Vector))+"::vector")
		if includeMeta {
			vals = append(vals, bind(r.MetaJSON)+"::jsonb")
		}
		if withCollection {
			vals = append(vals, bind(collectionID)+"::uuid")
		}
		b.WriteString("(" + strings.Join(vals, ", ") + ")")
	}

	sets := []string{
		cols.QContent + " = EXCLUDED." + cols.QContent,
		cols.QVector + " = EXCLUDED." + cols.QVector,
	}
	if includeMeta {
		sets = append(sets, cols.QMetadata+" = EXCLUDED."+cols.QMetadata)
	}
	if withCollection {
		sets = append(sets, pgvector.QuotedIDColumn()+" = EXCLUDED."+pgvector.QuotedIDColumn())
	}

	b.WriteString(" ON CONFLICT (" + target + ") DO UPDATE SET " + strings.Join(sets, ", "))
	b.WriteString(" RETURNING " + cols.QID)

	return b.String(), args
}

// upsertError explains the three failures that are specific to upserting, and
// hands everything else back to the shared humaniser.
func upsertError(auth pgvector.Auth, err error, label, matchBy, metaKey string, cols pgvector.ColumnSet, suffixed bool) string {
	var pe *pq.Error
	if errors.As(err, &pe) {
		switch pe.Code {
		case "42P10": // invalid_column_reference: nothing unique to conflict on
			if metaKey != "" {
				return fmt.Sprintf(
					"Upserting by the metadata field %q needs a unique index on it. Ask your DBA to run: "+
						"CREATE UNIQUE INDEX ON %s ((%s->>'%s'));",
					metaKey, label, cols.Metadata, metaKey)
			}
			return fmt.Sprintf(
				"Upserting by ID needs the %q column on %s to be the primary key, or to have a unique index. "+
					"Ask your DBA to run: CREATE UNIQUE INDEX ON %s (%s);",
				cols.ID, label, label, cols.ID)

		case "23502": // not_null_violation
			if pe.Column == cols.ID || (pe.Column == "" && strings.Contains(pe.Message, `"`+cols.ID+`"`)) {
				return fmt.Sprintf(
					"The %q column on %s doesn't fill itself in, so every document needs its own ID — set the ID "+
						"field, or give each document an \"id\" in the Documents list", cols.ID, label)
			}

		case "22P02": // invalid_text_representation
			if suffixed {
				return fmt.Sprintf(
					"Chunk Size splits a document into several rows, and each one is stored under the document's ID "+
						"with a #1, #2… on the end — which the %q column on %s won't accept. Use a text ID column, "+
						"or turn Chunk Size off", cols.ID, label)
			}
		}
	}
	return pgvector.Humanise(auth, err)
}

// normaliseID renders a returned key as something a later step can use. lib/pq
// hands back the types it has no decoder for (uuid among them) as raw bytes,
// which would otherwise reach the flow as a base64 blob.
func normaliseID(v interface{}) interface{} {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

func summary(docs []doc, rows []row, ids []interface{}, label, matched string) string {
	switch {
	case len(ids) == 1 && len(docs) == 1:
		return fmt.Sprintf("Upserted 1 document into %s, matching on %s: %s",
			label, matched, pgvector.Preview(rows[0].Content))
	case len(rows) > len(docs):
		return fmt.Sprintf("Upserted %d documents into %s as %d chunks, matching on %s",
			len(docs), label, len(ids), matched)
	default:
		return fmt.Sprintf("Upserted %d documents into %s, matching on %s", len(ids), label, matched)
	}
}
