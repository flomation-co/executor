package vectordatabase_pgvector_document_insert

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	ai_common "flomation.app/automate/executor/actions/ai"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
)

const (
	Author       = "Ethan Tan"
	Organisation = "Flomation"
	Name         = "Insert Documents"
	Description  = "Add documents to the vector store, embedding the text automatically"
	Website      = "https://www.flomation.co"
	Icon         = "database+plus"
	Date         = "13/07/2026"
	Type         = core.ActionTypeAction
)

// maxVectorDimensions is pgvector's own ceiling for a `vector` column. It is
// checked before the dimension is interpolated into a CREATE TABLE, so the only
// integer that ever reaches the SQL text is one the extension would accept.
const maxVectorDimensions = 16000

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Database Host", Placeholder: "db.example.com or 192.168.1.20 — hostname or IP, no scheme", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "5432"},
	{Name: "database", Type: core.ConnectionTypeString, Label: "Database", Placeholder: "vectordb", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "postgres", Required: true},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "Database password", Required: true},
	{Name: "ssl_mode", Type: core.ConnectionTypeString, Label: "SSL Mode", Placeholder: "disable", Options: pgvector.SSLModeOptions},
	{Name: "schema", Type: core.ConnectionTypeString, Label: "Schema", Placeholder: "public"},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table", Placeholder: "documents", Required: true},

	{Name: "id_column", Type: core.ConnectionTypeString, Label: "ID Column", Placeholder: "Leave empty to work it out from the table"},
	{Name: "content_column", Type: core.ConnectionTypeString, Label: "Content Column", Placeholder: "Leave empty to work it out from the table"},
	{Name: "metadata_column", Type: core.ConnectionTypeString, Label: "Metadata Column", Placeholder: "Leave empty to work it out from the table"},
	{Name: "vector_column", Type: core.ConnectionTypeString, Label: "Embedding Column", Placeholder: "Leave empty to work it out from the table"},

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

	{Name: "content", Type: core.ConnectionTypeText, Label: "Content", Placeholder: "The document text to store and make searchable"},
	{Name: "metadata", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `{"source": "handbook.pdf", "page": 3}`},
	{Name: "id", Type: core.ConnectionTypeString, Label: "ID", Placeholder: "Leave empty to let the database generate one"},
	{Name: "documents", Type: core.ConnectionTypeObject, Label: "Documents (JSON array)", Placeholder: `Bulk insert: [{"content": "…", "metadata": {…}, "id": "…", "embedding": […]}] — overrides the single Content field above`},
	{Name: "chunk_size", Type: core.ConnectionTypeInteger, Label: "Chunk Size (characters)", Placeholder: "0 = store the document whole. Set e.g. 1000 to split long text into overlapping chunks"},
	{Name: "chunk_overlap", Type: core.ConnectionTypeInteger, Label: "Chunk Overlap (characters)", Placeholder: "Characters of overlap between chunks — only used when Chunk Size is set"},
	{Name: "ensure_table", Type: core.ConnectionTypeBoolean, Label: "Create the table if it doesn't exist", Placeholder: "Off by default: a typo in the table name would otherwise build a second, empty table"},
	{Name: "collection", Type: core.ConnectionTypeString, Label: "Collection", Placeholder: "Optional — tag these documents as part of a named collection within the table"},
	{Name: "collection_table", Type: core.ConnectionTypeString, Label: "Collection Table", Placeholder: "flomation_vector_collections — where collection names are recorded"},
}

var Outputs = [...]core.Connection{
	{Name: "ids", Type: core.ConnectionTypeObject, Label: "Inserted IDs"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "First Inserted ID"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Rows Inserted"},
	{Name: "chunks", Type: core.ConnectionTypeInteger, Label: "Chunks"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// document is one row-to-be.
type document struct {
	Content   string
	Metadata  map[string]interface{}
	ID        string    // "" means let the column's DEFAULT fire
	Embedding []float32 // nil until embedded, or supplied per-document in the bulk list
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := pgvector.GetAuth(inputs)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if auth.Table == "" {
		return pgvector.Failf("Table is required — type the name of the table to insert into, or pick it from the dropdown")
	}
	label := auth.Schema + "." + auth.Table

	spec, err := pgvector.GetEmbedSpec(inputs)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	docs, err := readDocuments(inputs)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if len(docs) == 0 {
		return pgvector.Failf(
			"there's nothing to insert — put the document text in Content, or a list of documents in Documents")
	}
	if len(docs) > pgvector.MaxBatchDocuments {
		return pgvector.Failf(
			"that's %d documents in one step, and the limit is %d — feed them through in smaller batches",
			len(docs), pgvector.MaxBatchDocuments)
	}

	// Whether the *operator* asked to store metadata, captured before chunking
	// adds chunk_index of its own. A table with no metadata column is still a
	// perfectly good vector store, so only metadata they actually supplied is
	// worth failing over.
	suppliedMetadata := false
	for _, d := range docs {
		if len(d.Metadata) > 0 {
			suppliedMetadata = true
			break
		}
	}

	chunkSize := pgvector.OptionalInt(core.FindConnection("chunk_size", inputs), 0)
	chunkOverlap := pgvector.OptionalInt(core.FindConnection("chunk_overlap", inputs), 200)
	ensureTable := pgvector.OptionalBool(core.FindConnection("ensure_table", inputs), false)

	rows, err := expand(docs, chunkSize, chunkOverlap)
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

	exists, err := tableExists(ctx, db, auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if !exists && !ensureTable {
		return pgvector.Failf(
			"table %s doesn't exist, or this user can't see it. Table names are case-sensitive, so check the "+
				"spelling — or tick \"Create the table if it doesn't exist\" to have this step build it", label)
	}

	colIn := pgvector.ColumnInputs{
		ID:       pgvector.OptionalString(core.FindConnection("id_column", inputs)),
		Content:  pgvector.OptionalString(core.FindConnection("content_column", inputs)),
		Metadata: pgvector.OptionalString(core.FindConnection("metadata_column", inputs)),
		Vector:   pgvector.OptionalString(core.FindConnection("vector_column", inputs)),
	}

	// Resolve the table before embedding, not after: a table this node cannot
	// write to should fail before the operator has paid an embedding provider
	// for vectors that are about to be thrown away.
	var cols pgvector.ColumnSet
	declared := 0
	if exists {
		if cols, err = pgvector.ResolveColumns(ctx, db, auth.Schema, auth.Table, colIn); err != nil {
			return pgvector.Fail(auth, err)
		}
		if declared, err = pgvector.TableDimension(ctx, db, auth.Schema, auth.Table, cols.Vector); err != nil {
			return pgvector.Fail(auth, err)
		}
	}

	// One call for the whole batch. Embedding a hundred chunks in a loop is a
	// hundred HTTPS round-trips and, on OpenAI, a hundred times the chance of a
	// rate-limit; the provider takes an array for exactly this reason.
	pending := make([]int, 0, len(rows))
	texts := make([]string, 0, len(rows))
	for i := range rows {
		if len(rows[i].Embedding) == 0 {
			pending = append(pending, i)
			texts = append(texts, rows[i].Content)
		}
	}
	if len(texts) > 0 {
		vecs, err := spec.EmbedTexts(ctx, texts)
		if err != nil {
			return pgvector.ErrorResult(spec.EmbedError(err)), nil
		}
		if len(vecs) != len(texts) {
			return pgvector.Failf(
				"the embedding provider sent back %d embeddings for %d documents — they have to come back one for one",
				len(vecs), len(texts))
		}
		for i, v := range vecs {
			rows[pending[i]].Embedding = v
		}
	}

	dims := len(rows[0].Embedding)
	if dims == 0 {
		return pgvector.Failf("the embedding provider sent back an empty embedding — there is nothing to store")
	}
	for i := range rows {
		if len(rows[i].Embedding) != dims {
			return pgvector.Failf(
				"document %d has a %d-dimension embedding but the first one has %d — every row in a table has to "+
					"be the same size, so they can't all have come from the same embedding model",
				i+1, len(rows[i].Embedding), dims)
		}
	}

	if !exists {
		if err := createTable(ctx, db, auth.Schema, auth.Table, dims); err != nil {
			return pgvector.Fail(auth, err)
		}
		if cols, err = pgvector.ResolveColumns(ctx, db, auth.Schema, auth.Table, colIn); err != nil {
			return pgvector.Fail(auth, err)
		}
		declared = dims
	}

	// Preflight before anything is written, so a model that doesn't match the
	// table is a clean refusal rather than a driver error halfway down the batch.
	if err := pgvector.CheckDimension(declared, rows[0].Embedding, label); err != nil {
		return pgvector.Fail(auth, err)
	}
	if suppliedMetadata && !cols.HasMetadata() {
		return pgvector.Failf(
			"%s has no metadata column, so there's nowhere to put the metadata you supplied — add a jsonb column "+
				"to the table, or take the metadata back out", label)
	}

	// A collection stamps every inserted row with its id, provisioning the
	// collection table and the table's collection_id column the first time.
	collection := pgvector.GetCollection(inputs)
	collectionID := ""
	if collection.Active() {
		collectionID, err = collection.ResolveForWrite(ctx, db, auth.Schema, auth.Table)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
	}

	query, args, err := insertStatement(auth.Schema, auth.Table, cols, rows, collectionID)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	result, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	defer result.Close()

	ids := make([]string, 0, len(rows))
	for result.Next() {
		var raw interface{}
		if err := result.Scan(&raw); err != nil {
			return pgvector.Fail(auth, err)
		}
		ids = append(ids, scalarString(raw))
	}
	if err := result.Err(); err != nil {
		return pgvector.Fail(auth, err)
	}

	chunks := 0
	if chunkSize > 0 {
		chunks = len(rows)
	}
	first := ""
	if len(ids) > 0 {
		first = ids[0]
	}

	return pgvector.OK(map[string]interface{}{
		"ids":    ids,
		"id":     first,
		"count":  len(ids),
		"chunks": chunks,
		"result": map[string]interface{}{
			"table":      label,
			"inserted":   len(ids),
			"documents":  len(docs),
			"chunks":     chunks,
			"dimensions": dims,
			"ids":        ids,
		},
	}, summarise(label, docs, rows, chunkSize)), nil
}

// ---------------------------------------------------------------------------
// Reading the documents
// ---------------------------------------------------------------------------

// readDocuments resolves the two ways of saying what to insert.
//
// The bulk list wins outright when it has anything in it. Merging the two would
// be worse than either: an operator who filled in Content and then wired a
// Documents list from a loop would silently get an extra, stale row on every
// pass.
func readDocuments(inputs []*core.Connection) ([]document, error) {
	bulk, err := parseBulk(core.FindConnection("documents", inputs))
	if err != nil {
		return nil, err
	}
	if len(bulk) > 0 {
		return bulk, nil
	}

	content := pgvector.OptionalString(core.FindConnection("content", inputs))
	if content == "" {
		return nil, nil
	}
	meta, err := readMetadata(rawValue(core.FindConnection("metadata", inputs)))
	if err != nil {
		return nil, err
	}
	return []document{{
		Content:  content,
		Metadata: meta,
		ID:       pgvector.OptionalString(core.FindConnection("id", inputs)),
	}}, nil
}

func parseBulk(c *core.Connection) ([]document, error) {
	body, err := objectJSON(c)
	if err != nil || len(body) == 0 {
		return nil, err
	}

	// UseNumber so a large integer id (a Snowflake-style bigint, ~1.9e18)
	// survives decoding: a plain Unmarshal would round it through float64 and
	// write the wrong id. json.Number also keeps integer metadata as integers
	// when it is re-marshalled into the jsonb column.
	var list []map[string]interface{}
	if err := decodeJSONNumbers(body, &list); err != nil {
		// A single object rather than a list of them is a natural enough mistake
		// (and is what a one-item loop produces) that it is worth just accepting.
		var one map[string]interface{}
		if err2 := decodeJSONNumbers(body, &one); err2 != nil {
			return nil, fmt.Errorf(
				`Documents has to be a JSON list like [{"content": "…", "metadata": {…}}] — %v`, err)
		}
		list = []map[string]interface{}{one}
	}

	out := make([]document, 0, len(list))
	for i, e := range list {
		d := document{
			Content: firstString(e, "content", "text", "page_content"),
			ID:      scalarString(e["id"]),
		}

		meta, err := readMetadata(e["metadata"])
		if err != nil {
			return nil, fmt.Errorf("document %d: %v", i+1, err)
		}
		d.Metadata = meta

		// A document that arrives with its own embedding is taken at its word and
		// never re-embedded: that is the whole point of a pre-computed corpus, and
		// paying to recompute a vector the operator already has would be rude.
		if v, ok := e["embedding"]; ok && v != nil {
			vec, err := pgvector.CoerceVector(v)
			if err != nil {
				return nil, fmt.Errorf("document %d: %v", i+1, err)
			}
			d.Embedding = vec
		}

		out = append(out, d)
	}
	return out, nil
}

// readMetadata accepts a metadata value as an object or as the JSON text of one
// (which is what a ${...} reference resolves to).
func readMetadata(v interface{}) (map[string]interface{}, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case map[string]interface{}:
		return t, nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" || s == "null" || s == "{}" {
			return nil, nil
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(s), &m); err != nil {
			return nil, fmt.Errorf(
				`the Metadata isn't valid JSON: %v. It should look like {"source": "handbook.pdf", "page": 3}`, err)
		}
		return m, nil
	default:
		return nil, errors.New(
			`the Metadata has to be a JSON object like {"source": "handbook.pdf", "page": 3}`)
	}
}

// objectJSON renders an object-typed input as the JSON text it stands for.
//
// Connection.String() cannot be used on an object input: it falls through to a
// fmt.Sprintf("%v", …), which renders a map as Go's own map[k:v] syntax rather
// than as JSON. What actually lands in Value is either the decoded value (when
// it came straight from another action) or its JSON text (when it came through
// the ${...} substitution pass), and both have to work.
func objectJSON(c *core.Connection) ([]byte, error) {
	if c == nil || c.Value == nil {
		return nil, nil
	}
	switch v := c.Value.(type) {
	case string:
		s := strings.TrimSpace(v)
		if s == "" || s == "null" || s == "[]" || s == "{}" {
			return nil, nil
		}
		return []byte(s), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("couldn't read the Documents list: %v", err)
		}
		return b, nil
	}
}

func rawValue(c *core.Connection) interface{} {
	if c == nil {
		return nil
	}
	return c.Value
}

func firstString(e map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if s := scalarString(e[k]); s != "" {
			return s
		}
	}
	return ""
}

// scalarString renders whatever a JSON scalar (or a driver's RETURNING value)
// decoded to as text. An id column may be a uuid, a bigint or a text — lib/pq
// hands the first back as []byte — and all three have to come out as one string.
func scalarString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case []byte:
		return string(t)
	case json.Number:
		return t.String()
	case float64:
		return strconv.FormatFloat(t, 'f', -1, 64)
	case bool:
		return strconv.FormatBool(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// decodeJSONNumbers unmarshals with UseNumber so integers keep their exact
// value rather than being rounded through float64.
func decodeJSONNumbers(data []byte, into interface{}) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	return dec.Decode(into)
}

// ---------------------------------------------------------------------------
// Chunking
// ---------------------------------------------------------------------------

// expand turns the operator's documents into the rows that will actually be
// written, splitting each one when chunking is on.
func expand(docs []document, size, overlap int) ([]document, error) {
	if size <= 0 {
		for i, d := range docs {
			if d.Content == "" && len(d.Embedding) == 0 {
				return nil, fmt.Errorf("document %d has no content — there's nothing to embed or store", i+1)
			}
		}
		return docs, nil
	}

	out := make([]document, 0, len(docs))
	for i, d := range docs {
		if d.Content == "" && len(d.Embedding) == 0 {
			return nil, fmt.Errorf("document %d has no content — there's nothing to embed or store", i+1)
		}

		// A document that brought its own embedding describes itself whole. Cutting
		// it up would leave every piece but the first with no vector of its own, so
		// it is stored as it arrived.
		if len(d.Embedding) > 0 {
			out = append(out, d)
			continue
		}

		parts := chunkText(d.Content, size, overlap, pgvector.MaxBatchDocuments+1)
		if len(parts) > 1 && d.ID != "" {
			return nil, fmt.Errorf(
				"document %d splits into %d chunks but has a fixed ID, and every chunk would be written under that "+
					"same ID — clear the ID and let the database generate one per chunk, or set Chunk Size to 0",
				i+1, len(parts))
		}
		for n, part := range parts {
			out = append(out, document{
				Content:  part,
				Metadata: withChunkIndex(d.Metadata, n),
				ID:       d.ID,
			})
		}

		if len(out) > pgvector.MaxBatchDocuments {
			return nil, fmt.Errorf(
				"that splits into more than %d chunks, which is more than one step can write — raise Chunk Size, "+
					"or split the text upstream and feed it in a few documents at a time",
				pgvector.MaxBatchDocuments)
		}
	}
	return out, nil
}

// withChunkIndex copies a document's metadata and records which slice of the
// original this row is. Copying matters: every chunk of one document would
// otherwise share (and overwrite) the same map.
//
// It is dropped silently when the table has no metadata column — a table that
// cannot store metadata can still store chunks, and refusing the insert over a
// bookkeeping field the operator never asked for would be obtuse.
func withChunkIndex(meta map[string]interface{}, n int) map[string]interface{} {
	out := make(map[string]interface{}, len(meta)+1)
	for k, v := range meta {
		out[k] = v
	}
	out["chunk_index"] = n
	return out
}

// chunkText splits a document into overlapping windows.
//
// Chunking is what makes retrieval precise. One vector averaged over fifty pages
// of a staff handbook points at nothing in particular, so a search for "how much
// holiday do I get" scores the whole book rather than the paragraph that answers
// the question. Splitting first means each vector stands for one idea.
//
// Cuts land on a paragraph break where there is one, then a line break, then a
// space, and only fall back to a hard cut mid-word when the window contains none
// of those. The overlap carries the tail of each chunk into the next so that a
// sentence straddling a boundary is still whole somewhere.
func chunkText(s string, size, overlap, limit int) []string {
	// Characters, not bytes: cutting a UTF-8 sequence in half would corrupt the
	// text, and an operator counting "1000 characters" means runes.
	runes := []rune(s)
	if size <= 0 || len(runes) <= size {
		return []string{s}
	}
	if overlap < 0 {
		overlap = 0
	}
	// The walk must make forward progress, and an overlap at or above the chunk
	// size would step backwards for ever.
	if overlap >= size {
		overlap = size / 2
	}

	out := make([]string, 0, len(runes)/(size-overlap)+1)
	for start := 0; start < len(runes) && len(out) < limit; {
		if start+size >= len(runes) {
			if tail := strings.TrimSpace(string(runes[start:])); tail != "" {
				out = append(out, tail)
			}
			break
		}

		cut := boundary(runes, start, start+size)
		if chunk := strings.TrimSpace(string(runes[start:cut])); chunk != "" {
			out = append(out, chunk)
		}

		next := cut - overlap
		if next <= start {
			next = cut
		}
		start = next
	}
	if len(out) == 0 {
		return []string{s}
	}
	return out
}

// boundary picks where to cut, searching backwards from the end of the window.
//
// Anything found in the first half of the window is ignored: a paragraph break
// three characters in is a "natural" boundary that would produce a chunk holding
// no meaning at all, and a full-size hard cut is the better trade.
func boundary(runes []rune, start, end int) int {
	floor := start + (end-start)/2

	for i := end - 1; i > floor; i-- {
		if runes[i] == '\n' && runes[i-1] == '\n' {
			return i + 1
		}
	}
	for i := end - 1; i > floor; i-- {
		if runes[i] == '\n' {
			return i + 1
		}
	}
	for i := end - 1; i > floor; i-- {
		if runes[i] == ' ' {
			return i + 1
		}
	}
	return end
}

// ---------------------------------------------------------------------------
// SQL
// ---------------------------------------------------------------------------

// insertStatement builds one multi-row INSERT.
//
// One statement, not one per row: a batch of chunks is then a single round trip
// and a single transaction, so a failure halfway through a handbook leaves no
// half-written document behind to be found by a later search.
//
// Every value is bound. The only things interpolated are identifiers that came
// back pre-quoted from ResolveColumns/QuoteRelation and the $n counters.
func insertStatement(schema, table string, cols pgvector.ColumnSet, rows []document, collectionID string) (string, []interface{}, error) {
	rel, err := pgvector.QuoteRelation(schema, table)
	if err != nil {
		return "", nil, err
	}

	// The id column is named only when at least one row supplies one. Leaving it
	// out entirely is what lets the column's DEFAULT (a generated uuid, a
	// sequence) fire, which is the common case.
	withID := false
	for _, r := range rows {
		if r.ID != "" {
			withID = true
			break
		}
	}
	withCollection := collectionID != ""

	names := make([]string, 0, 5)
	if withID {
		names = append(names, cols.QID)
	}
	names = append(names, cols.QContent)
	if cols.HasMetadata() {
		names = append(names, cols.QMetadata)
	}
	names = append(names, cols.QVector)
	if withCollection {
		names = append(names, pgvector.QuotedIDColumn())
	}

	args := make([]interface{}, 0, len(rows)*5)
	tuples := make([]string, 0, len(rows))
	for _, r := range rows {
		slots := make([]string, 0, len(names))

		if withID {
			// DEFAULT is a keyword, not a value, so a row with no id of its own can
			// sit in the same statement as one that has: binding NULL here would
			// suppress the column's default and violate the primary key instead.
			if r.ID == "" {
				slots = append(slots, "DEFAULT")
			} else {
				args = append(args, r.ID)
				slots = append(slots, fmt.Sprintf("$%d", len(args)))
			}
		}

		args = append(args, r.Content)
		slots = append(slots, fmt.Sprintf("$%d", len(args)))

		if cols.HasMetadata() {
			meta := r.Metadata
			if meta == nil {
				meta = map[string]interface{}{}
			}
			b, err := json.Marshal(meta)
			if err != nil {
				return "", nil, fmt.Errorf("couldn't store the metadata: %v", err)
			}
			args = append(args, string(b))
			slots = append(slots, fmt.Sprintf("$%d::jsonb", len(args)))
		}

		// The vector goes in as pgvector's text form with an explicit cast, so the
		// one input carrying thousands of floats never touches the SQL text.
		args = append(args, pgvector.VectorLiteral(r.Embedding))
		slots = append(slots, fmt.Sprintf("$%d::vector", len(args)))

		if withCollection {
			args = append(args, collectionID)
			slots = append(slots, fmt.Sprintf("$%d::uuid", len(args)))
		}

		tuples = append(tuples, "("+strings.Join(slots, ", ")+")")
	}

	query := "INSERT INTO " + rel + " (" + strings.Join(names, ", ") + ") VALUES " +
		strings.Join(tuples, ", ") + " RETURNING " + cols.QID
	return query, args, nil
}

func tableExists(ctx context.Context, db *sql.DB, schema, table string) (bool, error) {
	var found int
	err := db.QueryRowContext(ctx, `
		SELECT 1
		  FROM pg_class     c
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = $1
		   AND c.relname = $2
		   AND c.relkind IN ('r', 'p', 'v', 'm', 'f')`, schema, table).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// createTable builds the LangChain shape — id/text/metadata/embedding — so that
// a table this node creates is the same one an existing n8n flow already reads.
//
// With one deliberate difference: the vector is dimensioned at whatever the
// embedding model actually produced, where n8n declares a bare `vector`. An
// unbounded vector column can never be given an ANN index, so an n8n-built table
// is condemned to a sequential scan over every row for the rest of its life. A
// dimensioned one can be indexed the moment it needs to be.
func createTable(ctx context.Context, db *sql.DB, schema, table string, dims int) error {
	if dims < 1 || dims > maxVectorDimensions {
		return fmt.Errorf(
			"the embedding model returned %d dimensions, and a vector column can hold between 1 and %d",
			dims, maxVectorDimensions)
	}
	rel, err := pgvector.QuoteRelation(schema, table)
	if err != nil {
		return err
	}

	// IF NOT EXISTS rather than a check-then-create: two runs of the same flow can
	// reach this line at once, and the loser of that race should carry on inserting
	// rather than fail on a table that now exists.
	_, err = db.ExecContext(ctx, fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			id        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			text      text,
			metadata  jsonb DEFAULT '{}'::jsonb,
			embedding vector(%d)
		)`, rel, dims))
	return err
}

// ---------------------------------------------------------------------------
// Summary
// ---------------------------------------------------------------------------

func summarise(label string, docs, rows []document, chunkSize int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Inserted %d document%s into %s", len(docs), plural(len(docs)), label)
	if chunkSize > 0 && len(rows) != len(docs) {
		fmt.Fprintf(&b, ", split into %d chunks", len(rows))
	}
	if len(docs) == 1 && docs[0].Content != "" {
		fmt.Fprintf(&b, ": %s", pgvector.Preview(docs[0].Content))
	}
	return b.String()
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}
