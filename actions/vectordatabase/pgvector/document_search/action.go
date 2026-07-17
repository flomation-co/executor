package vectordatabase_pgvector_document_search

import (
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
	Name         = "Search Documents"
	Description  = "Find the documents most similar in meaning to a query"
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

	{Name: "id_column", Type: core.ConnectionTypeString, Label: "ID Column", Placeholder: "Leave empty to work it out from the table"},
	{Name: "content_column", Type: core.ConnectionTypeString, Label: "Content Column", Placeholder: "Leave empty to work it out from the table"},
	{Name: "metadata_column", Type: core.ConnectionTypeString, Label: "Metadata Column", Placeholder: "Leave empty to work it out from the table"},
	{Name: "vector_column", Type: core.ConnectionTypeString, Label: "Embedding Column", Placeholder: "Leave empty to work it out from the table"},

	{Name: "embedding_source", Type: core.ConnectionTypeString, Label: "Embedding Source", Required: true, Options: []core.ConnectionOption{{Name: "Embed the text for me", Value: "inline"}, {Name: "Use a vector from a previous step", Value: "vector"}}},
	{Name: "embedding", Type: core.ConnectionTypeObject, Label: "Embedding Vector", Placeholder: "Pick the Embedding output of an Embed Text step", Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"vector"}}},
	{Name: "provider", Type: core.ConnectionTypeString, Label: "Embedding Provider", Required: true, Options: []core.ConnectionOption{{Name: "OpenAI", Value: "openai"}, {Name: "OpenAI-compatible (Azure, vLLM, LocalAI, TEI…)", Value: "openai_compatible"}, {Name: "Azure OpenAI", Value: "azure_openai"}, {Name: "Ollama (self-hosted)", Value: "ollama"}, {Name: "AWS Bedrock (Titan)", Value: "bedrock"}}, Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Embedding API Key", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"openai", "openai_compatible", "azure_openai"}}},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Embedding Base URL", Placeholder: "http://ollama.internal:11434 — Azure OpenAI: https://my-resource.openai.azure.com", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"openai_compatible", "ollama", "azure_openai"}}},
	{Name: "azure_api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "2024-10-21", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"azure_openai"}}},
	{Name: "access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key ID", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Access Key", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "AWS Region", Placeholder: "us-east-1", Visible: &core.VisibleWhen{Field: "provider", Values: []string{"bedrock"}}},
	{Name: "model", Type: core.ConnectionTypeComboBox, Label: "Embedding Model", Placeholder: "text-embedding-3-small", Options: []core.ConnectionOption{{Name: "OpenAI text-embedding-3-small (1536 dimensions)", Value: "text-embedding-3-small"}, {Name: "OpenAI text-embedding-3-large (3072 dimensions)", Value: "text-embedding-3-large"}, {Name: "OpenAI text-embedding-ada-002 (1536 dimensions)", Value: "text-embedding-ada-002"}, {Name: "Bedrock Titan Text v2 (1024 dimensions)", Value: "amazon.titan-embed-text-v2:0"}, {Name: "Bedrock Titan Text v1 (1536 dimensions)", Value: "amazon.titan-embed-text-v1"}, {Name: "Ollama nomic-embed-text (768 dimensions)", Value: "nomic-embed-text"}, {Name: "Ollama mxbai-embed-large (1024 dimensions)", Value: "mxbai-embed-large"}}, Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
	{Name: "dimensions", Type: core.ConnectionTypeInteger, Label: "Dimensions", Placeholder: "Leave empty for the model's default — must match the table", Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},

	{Name: "query_text", Type: core.ConnectionTypeText, Label: "Search Query", Placeholder: `What are you looking for? e.g. "how do I reset a password"`, Visible: &core.VisibleWhen{Field: "embedding_source", Values: []string{"inline"}}},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Number of Results", Placeholder: "4"},
	{Name: "distance_metric", Type: core.ConnectionTypeString, Label: "Distance Metric", Placeholder: "cosine", Options: []core.ConnectionOption{{Name: "Cosine — best for text embeddings", Value: "cosine"}, {Name: "Inner Product", Value: "inner_product"}, {Name: "Euclidean (L2)", Value: "euclidean"}}},
	{Name: "min_score", Type: core.ConnectionTypeString, Label: "Minimum Score", Placeholder: "0.0–1.0. Leave empty for no minimum. 0.7 is a good starting point for cosine"},
	{Name: "metadata_filter", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata Filter", Placeholder: "Only search documents whose metadata matches"},
	{Name: "metadata_filter_json", Type: core.ConnectionTypeObject, Label: "Advanced Metadata Filter (JSON)", Placeholder: `{"page": {"gt": 3}, "tag": {"in": ["a","b"]}}`},
	{Name: "include_metadata", Type: core.ConnectionTypeBoolean, Label: "Include Metadata", Placeholder: "Return each document's metadata alongside its text"},
	{Name: "include_vectors", Type: core.ConnectionTypeBoolean, Label: "Include Embeddings", Placeholder: "Return the raw embeddings too — usually not needed and very large"},
	{Name: "ef_search", Type: core.ConnectionTypeInteger, Label: "HNSW ef_search", Placeholder: "Leave empty for the server default. Higher = better recall, slower"},
	{Name: "collection", Type: core.ConnectionTypeString, Label: "Collection", Placeholder: "Optional — a named sub-set of the table to search within"},
	{Name: "collection_table", Type: core.ConnectionTypeString, Label: "Collection Table", Placeholder: "flomation_vector_collections — where collection names are recorded"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Results"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Result Count"},
	{Name: "top_result", Type: core.ConnectionTypeObject, Label: "Best Match"},
	{Name: "context", Type: core.ConnectionTypeText, Label: "Context"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

const (
	// defaultLimit matches n8n's PGVector retriever, so a flow ported across
	// returns the same number of chunks and the prompts built on it still fit.
	defaultLimit = 4

	// maxEfSearch is pgvector's own ceiling on hnsw.ef_search.
	maxEfSearch = 1000
)

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := pgvector.GetAuth(inputs)
	if err != nil {
		return pgvector.Failf("%s", err.Error())
	}
	if auth.Table == "" {
		return pgvector.Failf("Table is required — pick the table that holds your documents")
	}

	metricName := pgvector.OptionalString(core.FindConnection("distance_metric", inputs))
	if metricName == "" {
		metricName = "cosine"
	}
	m, err := pgvector.Metric(metricName)
	if err != nil {
		return pgvector.Failf("%s", err.Error())
	}

	minScore, hasMinScore, err := minimumScore(inputs)
	if err != nil {
		return pgvector.Failf("%s", err.Error())
	}

	efSearch := pgvector.OptionalInt(core.FindConnection("ef_search", inputs), 0)
	if efSearch < 0 || efSearch > maxEfSearch {
		return pgvector.Failf("HNSW ef_search has to be between 1 and %d — leave it empty to use the server's own setting", maxEfSearch)
	}

	limit := pgvector.Clamp(pgvector.OptionalInt(core.FindConnection("limit", inputs), defaultLimit), defaultLimit, pgvector.MaxRows)
	includeMetadata := pgvector.OptionalBool(core.FindConnection("include_metadata", inputs), true)
	includeVectors := pgvector.OptionalBool(core.FindConnection("include_vectors", inputs), false)

	embed, err := pgvector.GetEmbedSpec(inputs)
	if err != nil {
		return pgvector.Failf("%s", err.Error())
	}
	advanced, err := objectJSON(core.FindConnection("metadata_filter_json", inputs))
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
		return pgvector.Failf("%s", err.Error())
	}

	collection := pgvector.GetCollection(inputs)
	scope, err := collection.ResolveForRead(ctx, db, auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if collection.Active() && !scope.Exists {
		// Nothing has ever been written to this collection, so it has nothing to
		// search — and there is no point paying an embedding provider to find out.
		return pgvector.OK(emptySearch(),
			fmt.Sprintf("There's no collection named %q in %s.%s yet, so there's nothing to search.",
				collection.Name, auth.Schema, auth.Table)), nil
	}

	// The query vector is $1; the collection filter (if any) is $2; so the
	// metadata filter's placeholders start after both.
	startArg := 1
	if scope.Exists {
		startArg = 2
	}
	filter, err := pgvector.BuildFilter(cols, []*core.Connection{core.FindConnection("metadata_filter", inputs)}, advanced, startArg)
	if err != nil {
		return pgvector.Failf("%s", err.Error())
	}

	declared, err := pgvector.TableDimension(ctx, db, auth.Schema, auth.Table, cols.Vector)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	// Embedding happens after the table has been validated: a misspelled table
	// name should not cost the operator a call to a paid embedding provider.
	queryText := pgvector.OptionalString(core.FindConnection("query_text", inputs))
	vec, err := embed.EmbedOne(ctx, queryText)
	if err != nil {
		return pgvector.Failf("%s", embed.EmbedError(err))
	}
	if len(vec) == 0 {
		return pgvector.Failf("the search query came back with an empty embedding — check the embedding model")
	}
	label := auth.Schema + "." + auth.Table
	if err := pgvector.CheckDimension(declared, vec, label); err != nil {
		return pgvector.Failf("%s", err.Error())
	}

	withMetadata := includeMetadata && cols.HasMetadata()

	selected := []string{cols.QID, cols.QContent}
	if withMetadata {
		selected = append(selected, cols.QMetadata)
	}
	if includeVectors {
		selected = append(selected, cols.QVector)
	}
	distanceExpr := cols.QVector + " " + m.Operator + " $1::vector"
	selected = append(selected, distanceExpr+" AS _distance")

	args := []interface{}{pgvector.VectorLiteral(vec)}
	query := "SELECT " + strings.Join(selected, ", ") + " FROM " + relation
	// A row whose embedding is NULL — an external insert, or a document sitting
	// in an embed-async backfill window — has a NULL distance to any query
	// vector (pgvector's operators are strict). NULLs sort last under ORDER BY
	// ASC, so on a small or narrowly-filtered table they land inside the LIMIT
	// and their NULL _distance breaks the float scan. Exclude them: a row with
	// no embedding is not a search result. hybrid_search does the same.
	where := []string{cols.QVector + " IS NOT NULL"}
	if scope.Exists {
		clause, err := scope.ReadClause(&args)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		where = append(where, clause)
	}
	if filter.SQL != "" {
		where = append(where, filter.SQL)
		args = append(args, filter.Args...)
	}
	query += " WHERE " + strings.Join(where, " AND ")
	// ORDER BY the operator expression, ascending, is the only form an HNSW or
	// IVFFlat index can serve — order by the score instead (or descending) and
	// the plan silently degrades to a sequential scan over every row in the table.
	args = append(args, limit)
	query += " ORDER BY " + distanceExpr + " ASC LIMIT $" + strconv.Itoa(len(args))

	// One pinned session, because ef_search is session state: the pool keeps no
	// idle connections, so a SET issued against the pool would land on a
	// connection that is closed before the search ever runs.
	conn, err := db.Conn(ctx)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	defer conn.Close()

	if efSearch > 0 {
		if _, err := conn.ExecContext(ctx, "SET hnsw.ef_search = "+strconv.Itoa(efSearch)); err != nil {
			return pgvector.Failf("Couldn't set HNSW ef_search to %d — %s", efSearch, pgvector.Humanise(auth, err))
		}
	}

	rows, err := conn.QueryContext(ctx, query, args...)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0, limit)
	dropped := 0

	for rows.Next() {
		var (
			id       interface{}
			content  interface{}
			metadata interface{}
			embedded interface{}
			distance float64
		)
		targets := []interface{}{&id, &content}
		if withMetadata {
			targets = append(targets, &metadata)
		}
		if includeVectors {
			targets = append(targets, &embedded)
		}
		targets = append(targets, &distance)

		if err := rows.Scan(targets...); err != nil {
			return pgvector.Fail(auth, err)
		}

		// Both numbers, always, and each one always means the same thing.
		// distance is the raw operator output (lower is closer, and its range
		// depends on the metric); score is normalised so higher is better
		// whichever metric was used. n8n emits one number whose meaning flips
		// under the operator depending on a checkbox — a threshold set against
		// it means the opposite thing once reranking is switched on.
		row := map[string]interface{}{
			"id":       scalar(id),
			"content":  text(content),
			"distance": distance,
			"score":    pgvector.Similarity(metricName, distance),
		}
		if withMetadata {
			row["metadata"] = decodeMetadata(metadata)
		}
		if includeVectors {
			v, err := pgvector.ParseVector(embedded)
			if err != nil {
				return pgvector.Failf("couldn't read the embedding stored on document %v — %s", scalar(id), err.Error())
			}
			row["embedding"] = v
		}

		// The threshold is a function of the distance, so it cannot live in the
		// WHERE clause without defeating the index; the rows are already ordered
		// best-first, so filtering here just trims the tail.
		if hasMinScore && row["score"].(float64) < minScore {
			dropped++
			continue
		}
		results = append(results, row)
	}
	if err := rows.Err(); err != nil {
		return pgvector.Fail(auth, err)
	}

	// The whole point of a search step in a RAG flow: one block of plain text
	// that drops straight into an AI step's prompt, with no glue in between.
	var block strings.Builder
	for i, r := range results {
		if i > 0 {
			block.WriteString("\n\n")
		}
		fmt.Fprintf(&block, "[%d] %s", i+1, r["content"])
	}
	contextBlock := block.String()

	var top interface{}
	if len(results) > 0 {
		top = results[0]
	}

	// tool_result IS the context, so an agent calling this as a tool gets the
	// documents themselves back rather than a sentence about them.
	summary := contextBlock
	switch {
	case len(results) == 0 && dropped > 0:
		summary = fmt.Sprintf(
			"Nothing scored %s or better. %d document(s) matched but were less relevant than that — "+
				"lower the Minimum Score to see them.", formatScore(minScore), dropped)
	case len(results) == 0:
		summary = fmt.Sprintf("Nothing in %s matched that search.", label)
		if filter.SQL != "" {
			summary += " The metadata filter may be ruling everything out."
		}
	case dropped > 0:
		summary += fmt.Sprintf(
			"\n\n(%d further document(s) matched but scored below %s.)", dropped, formatScore(minScore))
	}

	return pgvector.OK(map[string]interface{}{
		"results":    results,
		"count":      len(results),
		"top_result": top,
		"context":    contextBlock,
	}, summary), nil
}

// emptySearch is the zero-result success payload, shaped like a real search so
// a downstream node sees the same fields whether or not anything matched.
func emptySearch() map[string]interface{} {
	return map[string]interface{}{
		"results":    []map[string]interface{}{},
		"count":      0,
		"top_result": nil,
		"context":    "",
	}
}

// minimumScore reads the optional relevance threshold. It is a String input
// rather than a number because "unset" and "0" have to be different answers:
// zero is a real threshold for inner product, where scores go negative.
func minimumScore(inputs []*core.Connection) (float64, bool, error) {
	raw := pgvector.OptionalString(core.FindConnection("min_score", inputs))
	if raw == "" {
		return 0, false, nil
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, false, fmt.Errorf(
			"Minimum Score %q isn't a number — use something like 0.7, or leave it empty to keep every result", raw)
	}
	return v, true, nil
}

func formatScore(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

// objectJSON reads an Object-typed input as JSON text.
//
// A filter typed into the editor arrives as a string, but the same field fed by
// a whole-value ${...} reference arrives as a real map — and Connection.String()
// renders that with fmt's %v, giving "map[page:map[gt:3]]", which is not JSON and
// would be rejected as a malformed filter.
func objectJSON(c *core.Connection) (string, error) {
	if c == nil || c.Value == nil {
		return "", nil
	}
	if s, ok := c.Value.(string); ok {
		return strings.TrimSpace(s), nil
	}
	b, err := json.Marshal(c.Value)
	if err != nil {
		return "", fmt.Errorf("couldn't read the Advanced Metadata Filter: %v", err)
	}
	return string(b), nil
}

// decodeMetadata turns the jsonb column into a real object, so a downstream step
// can reference ${search.results[0].metadata.source} rather than parsing a string.
// Anything that will not decode is handed back as text rather than dropped.
func decodeMetadata(src interface{}) interface{} {
	var raw []byte
	switch v := src.(type) {
	case nil:
		return nil
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		return v
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return string(raw)
	}
	return out
}

// scalar normalises a driver value for the flow store. lib/pq hands back every
// text-ish type — including uuid, which is what a LangChain-built table uses for
// its id — as []byte, and a []byte serialises to base64 in JSON.
func scalar(src interface{}) interface{} {
	if b, ok := src.([]byte); ok {
		return string(b)
	}
	return src
}

func text(src interface{}) string {
	switch v := src.(type) {
	case nil:
		return ""
	case []byte:
		return string(v)
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
