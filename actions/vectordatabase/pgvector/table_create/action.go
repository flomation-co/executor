package vectordatabase_pgvector_table_create

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
	"github.com/lib/pq"
)

const (
	Author       = "Ethan Tan"
	Organisation = "Flomation"
	Name         = "Create Vector Table"
	Description  = "Create a table that stores documents and their embeddings, with the index that makes search fast"
	Website      = "https://www.flomation.co"
	Icon         = "database+plus"
	Date         = "13/07/2026"
	Type         = core.ActionTypeAction
)

const (
	// maxDimensions is pgvector's hard ceiling on a `vector` column.
	maxDimensions = 16000

	// maxIndexableDimensions is the ceiling on an *indexable* column: both hnsw
	// and ivfflat refuse to build above 2000. A wider column is still perfectly
	// storable and searchable — it just has to be scanned — so this caps the
	// index, not the table.
	maxIndexableDimensions = 2000

	// maxIdentBytes is NAMEDATALEN-1. Postgres truncates a longer identifier
	// silently; QuoteIdent rejects it outright, so a composed index name is cut
	// here rather than blowing up on a long table name.
	maxIdentBytes = 63

	// hnswOptions and ivfflatOptions are pgvector's own documented starting
	// points. They are constants rather than inputs because an operator who has
	// an opinion about ef_construction is not the operator this node is for.
	hnswOptions    = "WITH (m = 16, ef_construction = 64)"
	ivfflatOptions = "WITH (lists = 100)"
)

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Database Host", Placeholder: "db.example.com or 192.168.1.20 — hostname or IP, no scheme", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "5432"},
	{Name: "database", Type: core.ConnectionTypeString, Label: "Database", Placeholder: "vectordb", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "postgres", Required: true},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "Database password", Required: true},
	{Name: "ssl_mode", Type: core.ConnectionTypeString, Label: "SSL Mode", Placeholder: "disable", Options: []core.ConnectionOption{{Name: "Disable — no encryption", Value: "disable"}, {Name: "Allow", Value: "allow"}, {Name: "Prefer — encrypt if the server offers it", Value: "prefer"}, {Name: "Require — encrypt, but don't verify the certificate", Value: "require"}, {Name: "Verify CA — encrypt and check the certificate authority", Value: "verify-ca"}, {Name: "Verify Full — encrypt and check the hostname too", Value: "verify-full"}}},
	{Name: "schema", Type: core.ConnectionTypeString, Label: "Schema", Placeholder: "public"},
	// Free text, not the table dropdown every other action gets: the whole point
	// of this step is that the table isn't there yet.
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table", Placeholder: "my_documents", Required: true},
	{Name: "vector_dimensions", Type: core.ConnectionTypeInteger, Label: "Embedding Dimensions", Placeholder: "1536 for OpenAI text-embedding-3-small, 1024 for Bedrock Titan v2", Required: true},
	{Name: "id_column", Type: core.ConnectionTypeString, Label: "ID Column", Placeholder: "id"},
	{Name: "content_column", Type: core.ConnectionTypeString, Label: "Content Column", Placeholder: "text"},
	{Name: "metadata_column", Type: core.ConnectionTypeString, Label: "Metadata Column", Placeholder: "metadata"},
	{Name: "vector_column", Type: core.ConnectionTypeString, Label: "Embedding Column", Placeholder: "embedding"},
	{Name: "create_extension", Type: core.ConnectionTypeBoolean, Label: "Create the pgvector extension if it's missing", Placeholder: "On by default — needs a database user that can install extensions"},
	{Name: "create_index", Type: core.ConnectionTypeBoolean, Label: "Create a search index", Placeholder: "On by default — without it every search reads the whole table"},
	{
		Name:        "index_type",
		Type:        core.ConnectionTypeString,
		Label:       "Index Type",
		Placeholder: "hnsw",
		Options: []core.ConnectionOption{
			{Name: "HNSW — fastest searches, slower to build", Value: "hnsw"},
			{Name: "IVFFlat — quicker to build, needs data to be loaded first", Value: "ivfflat"},
		},
		Visible: &core.VisibleWhen{Field: "create_index", Values: []string{"true"}},
	},
	{Name: "distance_metric", Type: core.ConnectionTypeString, Label: "Distance Metric", Placeholder: "cosine", Options: []core.ConnectionOption{{Name: "Cosine — best for text embeddings", Value: "cosine"}, {Name: "Inner Product", Value: "inner_product"}, {Name: "Euclidean (L2)", Value: "euclidean"}}},
	{Name: "enable_hybrid_search", Type: core.ConnectionTypeBoolean, Label: "Also index the text for keyword search", Placeholder: "Lets Hybrid Search match on exact words as well as meaning"},
}

var Outputs = [...]core.Connection{
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table"},
	{Name: "created", Type: core.ConnectionTypeBoolean, Label: "Created"},
	{Name: "dimensions", Type: core.ConnectionTypeInteger, Label: "Dimensions"},
	{Name: "index_name", Type: core.ConnectionTypeString, Label: "Index Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Table"},
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
		return pgvector.Failf("Table is required — give the new table a name, for example my_documents")
	}
	rel, err := pgvector.QuoteRelation(auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	label := auth.Schema + "." + auth.Table

	dims := pgvector.OptionalInt(core.FindConnection("vector_dimensions", inputs), 0)
	if dims < 1 || dims > maxDimensions {
		return pgvector.Failf(
			"Embedding Dimensions has to be a number between 1 and %d. It's how many numbers there are in one "+
				"embedding, and it comes from the model you embed with — OpenAI text-embedding-3-small is 1536, "+
				"Bedrock Titan v2 is 1024",
			maxDimensions)
	}

	// The LangChain/n8n column names, so a table this step creates is a drop-in
	// replacement for one built anywhere else in the ecosystem.
	idCol := defaulted(core.FindConnection("id_column", inputs), "id")
	contentCol := defaulted(core.FindConnection("content_column", inputs), "text")
	metaCol := defaulted(core.FindConnection("metadata_column", inputs), "metadata")
	vecCol := defaulted(core.FindConnection("vector_column", inputs), "embedding")

	qID, err := pgvector.QuoteIdent(idCol)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	qContent, err := pgvector.QuoteIdent(contentCol)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	qMeta, err := pgvector.QuoteIdent(metaCol)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	qVec, err := pgvector.QuoteIdent(vecCol)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if dup := duplicate(idCol, contentCol, metaCol, vecCol); dup != "" {
		return pgvector.Failf("%q is used for two different columns — each column needs its own name", dup)
	}

	metricName := pgvector.OptionalString(core.FindConnection("distance_metric", inputs))
	m, err := pgvector.Metric(metricName)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if metricName == "" {
		metricName = "cosine"
	}

	createExtension := pgvector.OptionalBool(core.FindConnection("create_extension", inputs), true)
	createIndex := pgvector.OptionalBool(core.FindConnection("create_index", inputs), true)
	hybrid := pgvector.OptionalBool(core.FindConnection("enable_hybrid_search", inputs), false)

	indexType := pgvector.OptionalString(core.FindConnection("index_type", inputs))
	if indexType == "" {
		indexType = "hnsw"
	}
	var indexOptions string
	switch indexType {
	case "hnsw":
		indexOptions = hnswOptions
	case "ivfflat":
		indexOptions = ivfflatOptions
	default:
		return pgvector.Failf("%q isn't an index type — choose HNSW or IVFFlat", indexType)
	}

	ctx, cancel := pgvector.Context(flow)
	defer cancel()

	db, err := pgvector.OpenConn(ctx, auth)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	defer db.Close()

	// Asked before the CREATE, because CREATE TABLE IF NOT EXISTS cannot tell us
	// afterwards whether it did anything — and "did this step just wipe the
	// table I spent an hour loading?" is the first thing an operator will want
	// to know. (It never does: nothing here drops or alters existing data.)
	existed, err := tableExists(ctx, db, auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Fail(auth, err)
	}

	var warnings []string
	storedDims := dims

	if existed {
		types, err := columnTypes(ctx, db, auth.Schema, auth.Table)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		udt, present := types[vecCol]
		switch {
		case !present:
			return pgvector.Failf(
				"there's already a table called %s and it has no %q column, so this step won't touch it. "+
					"Either pick a different table name, or set Embedding Column to the column that already holds "+
					"the embeddings",
				label, vecCol)
		case udt != "vector":
			return pgvector.Failf(
				"there's already a table called %s, and its %q column holds %s rather than embeddings. "+
					"Pick a different table name, or set Embedding Column to the column that holds the embeddings",
				label, vecCol, udt)
		}
		if hybrid {
			if _, present := types[contentCol]; !present {
				return pgvector.Failf(
					"keyword search needs the document text, but the existing table %s has no %q column — "+
						"set Content Column to the column that holds the text",
					label, contentCol)
			}
		}

		declared, err := pgvector.TableDimension(ctx, db, auth.Schema, auth.Table, vecCol)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		storedDims = declared
		if declared > 0 && declared != dims {
			warnings = append(warnings, fmt.Sprintf(
				"It stores %d-dimension embeddings, not the %d asked for here, and this step won't change that — "+
					"anything you put in it has to be %d-dimension.", declared, dims, declared))
		}
	}

	if createExtension {
		// IF NOT EXISTS makes this a no-op — and crucially a *permitted* no-op —
		// when the DBA has already installed the extension, which is the usual
		// case on a managed Postgres where nobody gets CREATE on the database.
		if _, err := db.ExecContext(ctx, "CREATE EXTENSION IF NOT EXISTS vector"); err != nil {
			if pgCode(err) == "42501" {
				return pgvector.Failf(
					"Your database user can't install the pgvector extension. Ask your DBA to run: " +
						"CREATE EXTENSION vector; — then run this step again.")
			}
			return pgvector.Fail(auth, err)
		}
	}

	// vector(N), not a bare `vector`. This one character is the difference
	// between a table Postgres can index and one it can only scan: an unbounded
	// vector column (which is what n8n's node creates) is rejected by both index
	// types, so every search against it reads every row for the rest of time.
	createTable := fmt.Sprintf(
		`CREATE TABLE IF NOT EXISTS %s (
			%s uuid NOT NULL DEFAULT gen_random_uuid() PRIMARY KEY,
			%s text,
			%s jsonb,
			%s vector(%d)
		)`, rel, qID, qContent, qMeta, qVec, dims)

	if _, err := db.ExecContext(ctx, createTable); err != nil {
		if pgCode(err) == "42883" && strings.Contains(err.Error(), "gen_random_uuid") {
			return pgvector.Failf(
				"This database can't generate document IDs on its own — gen_random_uuid() is built in from " +
					"PostgreSQL 13 onwards, and this server is older. Ask your DBA to run: " +
					"CREATE EXTENSION pgcrypto; — then run this step again.")
		}
		return pgvector.Fail(auth, err)
	}

	var indexName string
	if createIndex {
		switch {
		case storedDims < 0:
			// An existing column declared as a bare `vector`: the n8n legacy.
			warnings = append(warnings, fmt.Sprintf(
				"Its %q column was created without a fixed size, and pgvector can't index one of those, so "+
					"searches will read the whole table. To fix it, create a new table with this step and copy "+
					"the documents across.", vecCol))
		case storedDims > maxIndexableDimensions:
			warnings = append(warnings, fmt.Sprintf(
				"No index was created: pgvector can only index embeddings up to %d dimensions and these are %d, "+
					"so searches will read the whole table. Use a smaller embedding model, or set Dimensions on the "+
					"embedding step to shorten the vectors.", maxIndexableDimensions, storedDims))
		default:
			indexName = identifier(auth.Table, vecCol, indexType, "idx")
			qIndex, err := pgvector.QuoteIdent(indexName)
			if err != nil {
				return pgvector.Fail(auth, err)
			}
			// The ops class has to agree with the metric the searches will use.
			// Mismatch it and nothing breaks loudly — the index is simply never
			// chosen, and the operator is left with a slow node and no clue why.
			createIdx := fmt.Sprintf(
				`CREATE INDEX IF NOT EXISTS %s ON %s USING %s (%s %s) %s`,
				qIndex, rel, indexType, qVec, m.OpsClass, indexOptions)
			if _, err := db.ExecContext(ctx, createIdx); err != nil {
				return pgvector.Fail(auth, err)
			}
		}
	}

	var tsvIndexName string
	if hybrid {
		qTSV, err := pgvector.QuoteIdent("tsv")
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		// GENERATED ALWAYS ... STORED so the search vector can never drift out of
		// step with the text; a trigger-maintained column can, and every stale row
		// then quietly stops matching.
		addTSV := fmt.Sprintf(
			`ALTER TABLE %s ADD COLUMN IF NOT EXISTS %s tsvector
			 GENERATED ALWAYS AS (to_tsvector('english', coalesce(%s, ''))) STORED`,
			rel, qTSV, qContent)
		if _, err := db.ExecContext(ctx, addTSV); err != nil {
			return pgvector.Fail(auth, err)
		}

		tsvIndexName = identifier(auth.Table, "tsv", "idx")
		qTSVIndex, err := pgvector.QuoteIdent(tsvIndexName)
		if err != nil {
			return pgvector.Fail(auth, err)
		}
		createTSVIdx := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS %s ON %s USING gin (%s)`, qTSVIndex, rel, qTSV)
		if _, err := db.ExecContext(ctx, createTSVIdx); err != nil {
			return pgvector.Fail(auth, err)
		}
	}

	if warnings == nil {
		warnings = []string{}
	}
	result := map[string]interface{}{
		"table":           label,
		"schema":          auth.Schema,
		"created":         !existed,
		"dimensions":      storedDims,
		"id_column":       idCol,
		"content_column":  contentCol,
		"metadata_column": metaCol,
		"vector_column":   vecCol,
		"index_name":      indexName,
		"index_type":      indexType,
		"distance_metric": metricName,
		"hybrid_search":   hybrid,
		"keyword_index":   tsvIndexName,
		"warnings":        warnings,
	}

	return pgvector.OK(map[string]interface{}{
		"table":      label,
		"created":    !existed,
		"dimensions": storedDims,
		"index_name": indexName,
		"result":     result,
	}, summarise(label, existed, storedDims, indexName, tsvIndexName, warnings)), nil
}

// summarise writes the line the operator actually reads. It leads with whether
// the table is new, because that is the question they came with.
func summarise(label string, existed bool, dims int, indexName, tsvIndexName string, warnings []string) string {
	var b strings.Builder
	if existed {
		b.WriteString(fmt.Sprintf("Table %s already existed, so nothing in it was changed", label))
	} else if dims > 0 {
		b.WriteString(fmt.Sprintf("Created table %s for %d-dimension embeddings", label, dims))
	} else {
		b.WriteString(fmt.Sprintf("Created table %s", label))
	}

	var added []string
	if indexName != "" {
		added = append(added, "search index "+indexName)
	}
	if tsvIndexName != "" {
		added = append(added, "keyword index "+tsvIndexName)
	}
	if len(added) > 0 {
		b.WriteString(". Ready with " + strings.Join(added, " and "))
	}
	b.WriteString(".")

	for _, w := range warnings {
		b.WriteString(" " + w)
	}
	return b.String()
}

// defaulted reads an optional column-name input, falling back to the
// LangChain/n8n name.
func defaulted(c *core.Connection, def string) string {
	if v := pgvector.OptionalString(c); v != "" {
		return v
	}
	return def
}

// duplicate returns the first name used twice. Postgres would reject the CREATE
// anyway, but "column specified more than once" doesn't tell an operator which
// of the four boxes they typed the same thing into.
func duplicate(names ...string) string {
	seen := make(map[string]struct{}, len(names))
	for _, n := range names {
		if _, ok := seen[n]; ok {
			return n
		}
		seen[n] = struct{}{}
	}
	return ""
}

// identifier composes an index name from parts that have each already been
// through QuoteIdent, so every byte is ASCII and cutting at maxIdentBytes cannot
// land mid-character. Postgres would truncate it silently; doing it here keeps
// the name we report in index_name equal to the name that exists.
func identifier(parts ...string) string {
	name := strings.Join(parts, "_")
	if len(name) > maxIdentBytes {
		name = name[:maxIdentBytes]
	}
	return name
}

// pgCode extracts the SQLSTATE from a driver error, "" if it isn't one.
func pgCode(err error) string {
	var pe *pq.Error
	if errors.As(err, &pe) {
		return string(pe.Code)
	}
	return ""
}

// tableExists asks the catalog rather than trusting the CREATE, and matches
// ordinary and partitioned tables only — a view or a matview by that name is not
// something this step can write into.
func tableExists(ctx context.Context, db *sql.DB, schema, table string) (bool, error) {
	var found bool
	err := db.QueryRowContext(ctx, `
		SELECT EXISTS (
			SELECT 1
			  FROM pg_class     c
			  JOIN pg_namespace n ON n.oid = c.relnamespace
			 WHERE n.nspname = $1
			   AND c.relname = $2
			   AND c.relkind IN ('r', 'p')
		)`, schema, table).Scan(&found)
	if err != nil {
		return false, err
	}
	return found, nil
}

// columnTypes reads an existing table's columns so this step can say what is
// wrong with it before issuing a CREATE that would be a no-op and an index that
// would fail on a column name the operator never had.
func columnTypes(ctx context.Context, db *sql.DB, schema, table string) (map[string]string, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.attname, t.typname
		  FROM pg_attribute a
		  JOIN pg_class     c ON c.oid = a.attrelid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		  JOIN pg_type      t ON t.oid = a.atttypid
		 WHERE n.nspname = $1
		   AND c.relname = $2
		   AND a.attnum  > 0
		   AND NOT a.attisdropped`, schema, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string]string{}
	for rows.Next() {
		var name, udt string
		if err := rows.Scan(&name, &udt); err != nil {
			return nil, err
		}
		out[name] = udt
	}
	return out, rows.Err()
}
