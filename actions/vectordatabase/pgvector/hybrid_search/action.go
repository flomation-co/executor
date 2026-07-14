package vectordatabase_pgvector_hybrid_search

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
)

// Hybrid search exists because meaning-based search alone quietly loses the one
// thing a front-of-house operator most often searches for: an exact token. A
// customer quoting order code "FLM-4021", an error number, a surname — an
// embedding smears all three into "something about an order", and the right
// document comes back fourth. Keyword search nails those and is hopeless at
// "my delivery never turned up" matching a page titled "Missing parcels".
//
// So this runs both and fuses the two ranked lists with Reciprocal Rank Fusion:
// each list contributes 1/(k + rank) to a document's score. RRF fuses on
// POSITION, never on the raw scores, which is the whole point — a cosine
// distance and a ts_rank_cd are not on the same scale and no amount of
// normalising makes them comparable. k (60 by convention) damps the top of each
// list so one search cannot dominate the other on its first hit alone.
//
// n8n has no equivalent node. Fusing the two searches is what production RAG
// actually does, and doing it inside one Postgres query — rather than pulling
// two result sets into the executor and merging them in Go — means the database
// does the work it is good at and one round trip pays for both halves.

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Hybrid Search"
	Description  = "Combine meaning-based and keyword search, so exact terms and product codes are not missed"
	Website      = "https://www.flomation.co"
	Icon         = "database+bolt"
	Date         = "13/07/2026"
	Type         = core.ActionTypeAction
)

const (
	defaultLimit      = 10
	defaultCandidates = 50

	// defaultRRFK is the constant every RRF paper and every pgvector hybrid
	// example uses. Raising it flattens the two lists towards equal weight;
	// lowering it lets each list's top hit dominate.
	defaultRRFK = 60
	maxRRFK     = 1000

	// boundBeforeFilter is how many parameters this query binds before the
	// metadata filter's own: the query vector, the candidate limit, the text
	// search language, the keyword query and the RRF constant. BuildFilter needs
	// it so its placeholders continue the sequence instead of restarting at $1.
	boundBeforeFilter = 5
)

// ftsLanguageOptions are kept to the two configurations that exist on every
// stock Postgres. The language IS bound (to_tsvector($n::regconfig, …)) rather
// than interpolated, so this list is about not offering an operator a
// configuration their server has never heard of — not about injection.
var ftsLanguageOptions = []core.ConnectionOption{
	{Name: "English — matches \"invoices\" to \"invoice\", ignores \"the\", \"and\"", Value: "english"},
	{Name: "Simple — match words exactly as typed", Value: "simple"},
}

var ftsLanguages = map[string]struct{}{"english": {}, "simple": {}}

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

	{Name: "query_text", Type: core.ConnectionTypeText, Label: "Search Query", Placeholder: "What are you looking for? e.g. my parcel never arrived"},
	{Name: "text_query", Type: core.ConnectionTypeText, Label: "Keyword Query", Placeholder: "Leave empty to use the Search Query above"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Number of Results", Placeholder: "10"},
	{Name: "distance_metric", Type: core.ConnectionTypeString, Label: "Distance Metric", Placeholder: "cosine", Options: []core.ConnectionOption{{Name: "Cosine — best for text embeddings", Value: "cosine"}, {Name: "Inner Product", Value: "inner_product"}, {Name: "Euclidean (L2)", Value: "euclidean"}}},
	{Name: "fts_language", Type: core.ConnectionTypeString, Label: "Text Search Language", Placeholder: "english", Options: ftsLanguageOptions},
	{Name: "rrf_k", Type: core.ConnectionTypeInteger, Label: "Fusion Constant (k)", Placeholder: "Reciprocal Rank Fusion constant — 60 is the standard"},
	{Name: "candidate_limit", Type: core.ConnectionTypeInteger, Label: "Candidates Per Search", Placeholder: "How many candidates each search contributes before they're fused"},
	{Name: "tsvector_column", Type: core.ConnectionTypeString, Label: "Indexed Text Column", Placeholder: "Leave empty to search the content column directly. Set this to a generated tsvector column for a much faster keyword search"},
	{Name: "metadata_filter", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata Filter", Placeholder: "Only search documents whose metadata matches, e.g. source = handbook"},
	{Name: "metadata_filter_json", Type: core.ConnectionTypeText, Label: "Advanced Metadata Filter (JSON)", Placeholder: `{"page": {"gt": 3}, "tag": {"in": ["a","b"]}}`},
	{Name: "include_metadata", Type: core.ConnectionTypeBoolean, Label: "Include Metadata"},
	{Name: "include_vectors", Type: core.ConnectionTypeBoolean, Label: "Include Embeddings", Placeholder: "Return the stored vector with each result — large, and rarely needed"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Results"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Result Count"},
	{Name: "top_result", Type: core.ConnectionTypeObject, Label: "Top Result"},
	{Name: "context", Type: core.ConnectionTypeText, Label: "Context"},
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
		return pgvector.Failf("Table is required — pick the table your documents are stored in")
	}

	embedder, err := pgvector.GetEmbedSpec(inputs)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	// One box for the common case, two for the operator who wants to spell the
	// keyword half differently ("FLM-4021" for the keyword search, "my order is
	// late" for the meaning-based one). Either box on its own drives both.
	queryText := pgvector.OptionalString(core.FindConnection("query_text", inputs))
	keyword := pgvector.OptionalString(core.FindConnection("text_query", inputs))
	if keyword == "" {
		keyword = queryText
	}
	embedText := queryText
	if embedText == "" {
		embedText = keyword
	}
	if keyword == "" {
		return pgvector.Failf("There's nothing to search for — fill in the Search Query")
	}

	metricName := pgvector.OptionalString(core.FindConnection("distance_metric", inputs))
	if metricName == "" {
		metricName = "cosine"
	}
	m, err := pgvector.Metric(metricName)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	language := pgvector.OptionalString(core.FindConnection("fts_language", inputs))
	if language == "" {
		language = "english"
	}
	if _, ok := ftsLanguages[language]; !ok {
		return pgvector.Failf("%q isn't a text search language this step can use — choose English or Simple", language)
	}

	limit := pgvector.Clamp(pgvector.OptionalInt(core.FindConnection("limit", inputs), 0), defaultLimit, pgvector.MaxRows)
	candidates := pgvector.Clamp(pgvector.OptionalInt(core.FindConnection("candidate_limit", inputs), 0), defaultCandidates, pgvector.MaxRows)
	// Fusing fewer candidates than the operator asked to see would cap the result
	// set below their own Number of Results, which looks like a bug to them.
	if candidates < limit {
		candidates = limit
	}
	rrfK := pgvector.Clamp(pgvector.OptionalInt(core.FindConnection("rrf_k", inputs), 0), defaultRRFK, maxRRFK)

	wantVectors := pgvector.OptionalBool(core.FindConnection("include_vectors", inputs), false)
	wantMetadata := pgvector.OptionalBool(core.FindConnection("include_metadata", inputs), true)

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
	wantMetadata = wantMetadata && cols.HasMetadata()

	relation, err := pgvector.QuoteRelation(auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	textSearch, failure, err := textSearchExpr(ctx, db, auth, cols, inputs)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if failure != "" {
		return pgvector.ErrorResult(failure), nil
	}

	vec, err := embedder.EmbedOne(ctx, embedText)
	if err != nil {
		return pgvector.Failf("%s", embedder.EmbedError(err))
	}

	declared, err := pgvector.TableDimension(ctx, db, auth.Schema, auth.Table, cols.Vector)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if err := pgvector.CheckDimension(declared, vec, auth.Schema+"."+auth.Table); err != nil {
		return pgvector.Fail(auth, err)
	}

	filter, err := pgvector.BuildFilter(
		cols,
		[]*core.Connection{core.FindConnection("metadata_filter", inputs)},
		pgvector.OptionalString(core.FindConnection("metadata_filter_json", inputs)),
		boundBeforeFilter,
	)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	// The filter's placeholders are bound once and referenced from BOTH CTEs —
	// Postgres is happy to see the same $n twice, and binding the values a second
	// time would only shift every later placeholder out of step.
	args := []interface{}{pgvector.VectorLiteral(vec), candidates, language, keyword, rrfK}
	args = append(args, filter.Args...)
	limitArg := boundBeforeFilter + len(filter.Args) + 1
	args = append(args, limit)

	query := buildQuery(queryPlan{
		Relation:   relation,
		Cols:       cols,
		Operator:   m.Operator,
		TextSearch: textSearch,
		Filter:     filter.SQL,
		LimitArg:   limitArg,
		Metadata:   wantMetadata,
		Vectors:    wantVectors,
	})

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	defer rows.Close()

	results := make([]map[string]interface{}, 0, limit)
	contexts := make([]string, 0, limit)
	fusedBoth := 0

	for rows.Next() && len(results) < pgvector.MaxRows {
		var (
			id         interface{}
			content    sql.NullString
			metaRaw    []byte
			vecRaw     interface{}
			distance   sql.NullFloat64
			score      float64
			vectorRank sql.NullInt64
			textRank   sql.NullInt64
		)
		targets := []interface{}{&id, &content}
		if wantMetadata {
			targets = append(targets, &metaRaw)
		}
		if wantVectors {
			targets = append(targets, &vecRaw)
		}
		targets = append(targets, &distance, &score, &vectorRank, &textRank)

		if err := rows.Scan(targets...); err != nil {
			return pgvector.Fail(auth, err)
		}

		row := map[string]interface{}{
			"id":          scalar(id),
			"content":     content.String,
			"score":       score,
			"distance":    nullFloat(distance),
			"vector_rank": nullInt(vectorRank),
			"text_rank":   nullInt(textRank),
		}
		// score is the fused rank, which says nothing about how close the document
		// actually is; similarity is the same 0..1 relevance the plain search
		// emits, so a threshold written against one node still reads here.
		if distance.Valid {
			row["similarity"] = pgvector.Similarity(metricName, distance.Float64)
		}
		if vectorRank.Valid && textRank.Valid {
			fusedBoth++
		}
		if wantMetadata {
			row["metadata"] = decodeMetadata(metaRaw)
		}
		if wantVectors {
			stored, err := pgvector.ParseVector(vecRaw)
			if err != nil {
				return pgvector.Fail(auth, err)
			}
			row["embedding"] = stored
		}

		results = append(results, row)
		if content.String != "" {
			contexts = append(contexts, content.String)
		}
	}
	if err := rows.Err(); err != nil {
		return pgvector.Fail(auth, err)
	}

	var top interface{}
	if len(results) > 0 {
		top = results[0]
	}

	return pgvector.OK(map[string]interface{}{
		"results":    results,
		"count":      len(results),
		"top_result": top,
		"context":    strings.Join(contexts, "\n\n"),
	}, summarise(results, fusedBoth, auth, embedText, keyword)), nil
}

// queryPlan is everything the SQL needs that is already validated and quoted.
type queryPlan struct {
	Relation   string
	Cols       pgvector.ColumnSet
	Operator   string
	TextSearch string // pre-quoted tsvector column, or a to_tsvector() expression
	Filter     string // "" when there is nothing to filter on
	LimitArg   int
	Metadata   bool
	Vectors    bool
}

// buildQuery assembles the fusion.
//
// Every value is a placeholder; the only things interpolated are identifiers
// that have already been through QuoteIdent/QuoteRelation, the distance operator
// (which comes from a fixed table, never from the operator) and $n numbers this
// file counts itself.
//
// The two CTEs each rank their own hits, then a UNION ALL + GROUP BY fuses them.
// That does the job of a FULL OUTER JOIN — a document found by only one of the
// two searches still appears, with the other rank left null — without having to
// COALESCE the id back out of two nullable sides.
//
// The kw CTE turns plainto_tsquery's ANDs into ORs, and it is not optional.
// plainto_tsquery("my customer is asking about FLM-4021 again") compiles to
// 'custom' & 'ask' & 'flm' & '-4021', and the document that actually contains
// FLM-4021 says nothing about customers or asking — so it does not match, and
// the keyword half goes silent at the exact moment it is the only half that can
// find the right answer. AND is a filter; fusion needs a ranker. OR-ed, every
// document containing any query term is a candidate, ts_rank_cd orders them by
// how many and how rare those terms were, and RRF fuses on that order — which is
// the whole point of ranking rather than scoring. The rewrite is safe: the
// operator's text is still bound to $4 and never reaches the SQL, plainto_tsquery
// has already normalised it to lexemes server-side, and the only operator that
// function can emit between them is the & this swaps.
//
// ROW_NUMBER() OVER (ORDER BY …) alongside the same ORDER BY … LIMIT is not the
// redundancy it looks like: the window numbers the rows, the LIMIT keeps the
// first N of them, and because both agree on the ordering, Postgres can still
// feed the whole thing straight from an ANN index and stop early.
func buildQuery(p queryPlan) string {
	var q strings.Builder

	q.WriteString("WITH kw AS (\n")
	q.WriteString("  SELECT replace(plainto_tsquery($3::regconfig, $4)::text, ' & ', ' | ')::tsquery AS query\n")
	q.WriteString("), v AS (\n")
	q.WriteString("  SELECT " + p.Cols.QID + " AS id,\n")
	q.WriteString("         ROW_NUMBER() OVER (ORDER BY " + p.Cols.QVector + " " + p.Operator + " $1::vector) AS rank\n")
	q.WriteString("    FROM " + p.Relation + "\n")
	q.WriteString("   WHERE " + p.Cols.QVector + " IS NOT NULL")
	q.WriteString(andFilter(p.Filter))
	q.WriteString("\n   ORDER BY " + p.Cols.QVector + " " + p.Operator + " $1::vector\n")
	q.WriteString("   LIMIT $2\n")

	q.WriteString("), t AS (\n")
	q.WriteString("  SELECT " + p.Cols.QID + " AS id,\n")
	q.WriteString("         ROW_NUMBER() OVER (ORDER BY ts_rank_cd(" + p.TextSearch + ", kw.query) DESC) AS rank\n")
	q.WriteString("    FROM " + p.Relation + ", kw\n")
	q.WriteString("   WHERE " + p.TextSearch + " @@ kw.query")
	q.WriteString(andFilter(p.Filter))
	q.WriteString("\n   ORDER BY ts_rank_cd(" + p.TextSearch + ", kw.query) DESC\n")
	q.WriteString("   LIMIT $2\n")

	q.WriteString("), fused AS (\n")
	q.WriteString("  SELECT id,\n")
	q.WriteString("         SUM(1.0 / ($5::int + rank))::float8 AS score,\n")
	q.WriteString("         MAX(CASE WHEN src = 'v' THEN rank END) AS vector_rank,\n")
	q.WriteString("         MAX(CASE WHEN src = 't' THEN rank END) AS text_rank\n")
	q.WriteString("    FROM (SELECT id, rank, 'v' AS src FROM v\n")
	q.WriteString("           UNION ALL\n")
	q.WriteString("          SELECT id, rank, 't' AS src FROM t) ranked\n")
	q.WriteString("   GROUP BY id\n")
	q.WriteString(")\n")

	q.WriteString("SELECT b." + p.Cols.QID + ", b." + p.Cols.QContent)
	if p.Metadata {
		q.WriteString(", b." + p.Cols.QMetadata)
	}
	if p.Vectors {
		q.WriteString(", b." + p.Cols.QVector)
	}
	q.WriteString(",\n       b." + p.Cols.QVector + " " + p.Operator + " $1::vector AS distance,\n")
	q.WriteString("       f.score, f.vector_rank, f.text_rank\n")
	q.WriteString("  FROM fused f\n")
	q.WriteString("  JOIN " + p.Relation + " b ON b." + p.Cols.QID + " = f.id\n")
	q.WriteString(" ORDER BY f.score DESC, f.vector_rank ASC NULLS LAST\n")
	q.WriteString(" LIMIT $" + strconv.Itoa(p.LimitArg))

	return q.String()
}

func andFilter(sqlFragment string) string {
	if sqlFragment == "" {
		return ""
	}
	// Parenthesised because the filter may be an OR group, and it is being
	// AND-ed onto a predicate that is already there.
	return " AND (" + sqlFragment + ")"
}

// textSearchExpr decides what the keyword half actually searches.
//
// Given a stored tsvector column it searches that, which is the only way keyword
// search stays fast on a table of any size — a GIN index can serve it, and the
// stemming was paid for once at write time. Left blank, it builds the tsvector
// per row at query time, which is correct but reads every row.
//
// It returns (expression, operator-facing failure, unexpected error) so a
// misconfigured column can be explained rather than left to Postgres, whose
// answer is "column x does not exist" or a bare type error.
func textSearchExpr(ctx context.Context, db *sql.DB, auth pgvector.Auth, cols pgvector.ColumnSet, inputs []*core.Connection) (string, string, error) {
	name := pgvector.OptionalString(core.FindConnection("tsvector_column", inputs))
	if name == "" {
		// The language is bound, not spliced: to_tsvector($3::regconfig, …).
		return "to_tsvector($3::regconfig, coalesce(" + cols.QContent + ", ''))", "", nil
	}

	quoted, err := pgvector.QuoteIdent(name)
	if err != nil {
		return "", err.Error(), nil
	}

	typeName, err := columnType(ctx, db, auth.Schema, auth.Table, name)
	if err != nil {
		return "", "", err
	}
	switch typeName {
	case "tsvector":
		// Whatever language this column was generated with wins over Text Search
		// Language, which now only shapes the query side. That is the operator's
		// call to make and mismatching them simply finds less.
		return quoted, "", nil
	case "":
		return "", fmt.Sprintf(
			"the Indexed Text Column %q doesn't exist on %s.%s — leave it empty to search the content column instead",
			name, auth.Schema, auth.Table), nil
	default:
		return "", fmt.Sprintf(
			"the Indexed Text Column %q on %s.%s is a %s, not a tsvector — point it at a generated tsvector column, "+
				"or leave it empty to search the content column instead",
			name, auth.Schema, auth.Table, typeName), nil
	}
}

// columnType reads one column's type from the catalog, returning "" when the
// column is not there.
func columnType(ctx context.Context, db *sql.DB, schema, table, column string) (string, error) {
	var typeName string
	err := db.QueryRowContext(ctx, `
		SELECT t.typname
		  FROM pg_attribute a
		  JOIN pg_class     c ON c.oid = a.attrelid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		  JOIN pg_type      t ON t.oid = a.atttypid
		 WHERE n.nspname = $1
		   AND c.relname = $2
		   AND a.attname = $3
		   AND a.attnum  > 0
		   AND NOT a.attisdropped`, schema, table, column).Scan(&typeName)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return typeName, nil
}

// scalar renders an id the way the rest of the flow can use it. lib/pq hands
// back text, uuid and numeric ids as []byte, which would reach the editor as a
// base64 blob.
func scalar(v interface{}) interface{} {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	return v
}

func decodeMetadata(raw []byte) interface{} {
	if len(raw) == 0 {
		return nil
	}
	var out interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return string(raw)
	}
	return out
}

// nullFloat / nullInt keep a missing rank as a real null rather than a zero — a
// document the keyword search never found has NO text rank, and rank 0 would
// read as "it came first".
func nullFloat(v sql.NullFloat64) interface{} {
	if !v.Valid {
		return nil
	}
	return v.Float64
}

func nullInt(v sql.NullInt64) interface{} {
	if !v.Valid {
		return nil
	}
	return v.Int64
}

func summarise(results []map[string]interface{}, fusedBoth int, auth pgvector.Auth, embedText, keyword string) string {
	shown := embedText
	if shown == "" {
		shown = keyword
	}
	if len(results) == 0 {
		return fmt.Sprintf(
			"Nothing in %s.%s matched %q — neither the meaning-based search nor the keyword search found anything",
			auth.Schema, auth.Table, shown)
	}

	content, _ := results[0]["content"].(string)
	return fmt.Sprintf(
		"Found %d result(s) in %s.%s for %q — %d of them matched on both meaning and keywords. Top result: %s",
		len(results), auth.Schema, auth.Table, shown, fusedBoth, pgvector.Preview(content))
}
