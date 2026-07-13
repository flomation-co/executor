package vectordatabase_pgvector_index_create

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	pgvector "flomation.app/automate/executor/actions/vectordatabase/pgvector"
	"github.com/lib/pq"
)

const (
	Author       = "Ethan Tan"
	Organisation = "Flomation"
	Name         = "Create Search Index"
	Description  = "Add an approximate-nearest-neighbour index so similarity search stays fast as the table grows"
	Website      = "https://www.flomation.co"
	Icon         = "database+bolt"
	Date         = "13/07/2026"
	Type         = core.ActionTypeAction
)

// pgvector refuses to index a `vector` column wider than this. halfvec goes to
// 4000, at half the precision, which is the usual way out of it.
const maxIndexDimensions = 2000

// NAMEDATALEN - 1. Postgres truncates a longer index name itself, silently, and
// we would then be reporting a name that is not the one on the table.
const maxIdentBytes = 63

// pgvector's own bounds on the build parameters. Checking them here turns a
// driver error about an unrecognised parameter into a sentence that names the
// box the operator typed in.
const (
	minM              = 2
	maxM              = 100
	minEFConstruction = 4
	maxEFConstruction = 1000
	minLists          = 1
	maxLists          = 32768
)

// The operator class an index was built with is the only record of which
// distance metric it can serve, and it is what the "already exists" path has to
// compare against.
var opsClassMetric = map[string]string{
	"vector_cosine_ops": "cosine",
	"vector_ip_ops":     "inner product",
	"vector_l2_ops":     "euclidean",
}

var indexLabels = map[string]string{
	"hnsw":    "HNSW",
	"ivfflat": "IVFFlat",
}

var Inputs = [...]core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Database Host", Placeholder: "db.example.com or 192.168.1.20 — hostname or IP, no scheme", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "5432"},
	{Name: "database", Type: core.ConnectionTypeString, Label: "Database", Placeholder: "vectordb", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "postgres", Required: true},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "Database password", Required: true},
	{Name: "ssl_mode", Type: core.ConnectionTypeString, Label: "SSL Mode", Placeholder: "disable", Options: pgvector.SSLModeOptions},
	{Name: "schema", Type: core.ConnectionTypeString, Label: "Schema", Placeholder: "public"},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table", Placeholder: "documents", Required: true},
	{Name: "vector_column", Type: core.ConnectionTypeString, Label: "Embedding Column", Placeholder: "Leave blank to work it out from the table — set it if the table holds nothing but embeddings"},
	{
		Name:  "index_type",
		Type:  core.ConnectionTypeString,
		Label: "Index Type",
		Options: []core.ConnectionOption{
			{Name: "HNSW — best recall, slower to build", Value: "hnsw"},
			{Name: "IVFFlat — faster to build, needs data present", Value: "ivfflat"},
		},
	},
	{Name: "distance_metric", Type: core.ConnectionTypeString, Label: "Distance Metric", Placeholder: "Must match the metric your searches use, or they can't use the index", Options: pgvector.DistanceMetricOptions},
	{Name: "hnsw_m", Type: core.ConnectionTypeInteger, Label: "HNSW: Links per Row (m)", Placeholder: "16 — higher means better recall, a bigger index and a slower build", Visible: &core.VisibleWhen{Field: "index_type", Values: []string{"hnsw"}}},
	{Name: "hnsw_ef_construction", Type: core.ConnectionTypeInteger, Label: "HNSW: Build Effort (ef_construction)", Placeholder: "64 — higher means a slower build and better recall (at least twice m)", Visible: &core.VisibleWhen{Field: "index_type", Values: []string{"hnsw"}}},
	{Name: "ivfflat_lists", Type: core.ConnectionTypeInteger, Label: "IVFFlat: Lists", Placeholder: "100 — roughly rows ÷ 1000 for tables up to a million rows", Visible: &core.VisibleWhen{Field: "index_type", Values: []string{"ivfflat"}}},
	{Name: "concurrently", Type: core.ConnectionTypeBoolean, Label: "Build without locking the table", Placeholder: "Inserts and updates keep working while the index builds, but the build takes longer"},
	{Name: "analyze_after", Type: core.ConnectionTypeBoolean, Label: "Update Table Statistics", Placeholder: "Run ANALYZE afterwards so Postgres starts using the new index (recommended)"},
}

var Outputs = [...]core.Connection{
	{Name: "index_name", Type: core.ConnectionTypeString, Label: "Index Name"},
	{Name: "created", Type: core.ConnectionTypeBoolean, Label: "Created"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Index"},
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
		return pgvector.Failf("Table is required — this step needs to know which table to index")
	}

	indexType := pgvector.OptionalString(core.FindConnection("index_type", inputs))
	if indexType == "" {
		indexType = "hnsw"
	}
	if _, ok := indexLabels[indexType]; !ok {
		return pgvector.Failf("%q isn't an index type — use hnsw or ivfflat", indexType)
	}

	metricName := pgvector.OptionalString(core.FindConnection("distance_metric", inputs))
	if metricName == "" {
		metricName = "cosine"
	}
	// The operator class has to match the metric the searches use. Get this
	// wrong and Postgres still answers correctly — by reading every row — so
	// nothing looks broken except the speed, which is the only reason this
	// action exists.
	metric, err := pgvector.Metric(metricName)
	if err != nil {
		return pgvector.Failf("%s", err)
	}

	links := pgvector.OptionalInt(core.FindConnection("hnsw_m", inputs), 16)
	effort := pgvector.OptionalInt(core.FindConnection("hnsw_ef_construction", inputs), 64)
	lists := pgvector.OptionalInt(core.FindConnection("ivfflat_lists", inputs), 100)
	concurrently := pgvector.OptionalBool(core.FindConnection("concurrently", inputs), false)
	analyzeAfter := pgvector.OptionalBool(core.FindConnection("analyze_after", inputs), true)

	switch indexType {
	case "hnsw":
		if links < minM || links > maxM {
			return pgvector.Failf(
				"Links per Row (m) has to be between %d and %d — %d is outside what pgvector accepts",
				minM, maxM, links)
		}
		if effort < minEFConstruction || effort > maxEFConstruction {
			return pgvector.Failf(
				"Build Effort (ef_construction) has to be between %d and %d — %d is outside what pgvector accepts",
				minEFConstruction, maxEFConstruction, effort)
		}
		if effort < 2*links {
			return pgvector.Failf(
				"Build Effort (ef_construction) has to be at least twice Links per Row — with m set to %d it needs to be %d or more",
				links, 2*links)
		}
	case "ivfflat":
		if lists < minLists || lists > maxLists {
			return pgvector.Failf(
				"Lists has to be between %d and %d — %d is outside what pgvector accepts",
				minLists, maxLists, lists)
		}
	}

	qSchema, err := pgvector.QuoteIdent(auth.Schema)
	if err != nil {
		return pgvector.Failf("%s", err)
	}
	qRel, err := pgvector.QuoteRelation(auth.Schema, auth.Table)
	if err != nil {
		return pgvector.Failf("%s", err)
	}

	ctx, cancel := pgvector.Context(flow)
	defer cancel()

	db, err := pgvector.OpenConn(ctx, auth)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	defer db.Close()

	// CREATE INDEX only cares about the one column, but auto-detect is what lets
	// an operator point this at a table somebody else built without knowing
	// which column holds the embeddings.
	vectorColumn := pgvector.OptionalString(core.FindConnection("vector_column", inputs))
	if vectorColumn == "" {
		cols, err := pgvector.ResolveColumns(ctx, db, auth.Schema, auth.Table, pgvector.ColumnInputs{})
		if err != nil {
			return pgvector.Failf("%s", err)
		}
		vectorColumn = cols.Vector
	}
	qVector, err := pgvector.QuoteIdent(vectorColumn)
	if err != nil {
		return pgvector.Failf("%s", err)
	}

	dimensions, err := pgvector.TableDimension(ctx, db, auth.Schema, auth.Table, vectorColumn)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	switch {
	case dimensions == 0:
		return pgvector.Failf(
			"couldn't find a vector column called %q on %s.%s — check the spelling (names are case-sensitive), "+
				"or leave Embedding Column blank to work it out automatically",
			vectorColumn, auth.Schema, auth.Table)

	// An unbounded `vector` column — which is what n8n's own table builder
	// leaves behind — has no fixed size, and Postgres cannot build an ANN index
	// over a type whose width it doesn't know.
	case dimensions < 0:
		return pgvector.Failf(
			"The embedding column %q on %s.%s has no fixed size, so Postgres can't index it. That happens when the "+
				"table was created without a dimension. Recreate the table with Create Vector Table (which sets the "+
				"dimension), or run: ALTER TABLE %s ALTER COLUMN %s TYPE vector(1536);",
			vectorColumn, auth.Schema, auth.Table, qRel, qVector)

	case dimensions > maxIndexDimensions:
		return pgvector.Failf(
			"%s.%s stores %d-dimension vectors, and pgvector can only index up to %d of them. Embed with fewer "+
				"dimensions (OpenAI's text-embedding-3 models take a Dimensions setting, and 1536 indexes fine), or "+
				"store the embeddings as halfvec, which indexes up to 4000 at half the precision",
			auth.Schema, auth.Table, dimensions, maxIndexDimensions)
	}

	indexName := auth.Table + "_" + vectorColumn + "_" + indexType + "_idx"
	if len(indexName) > maxIdentBytes {
		indexName = indexName[:maxIdentBytes]
	}
	qIndex, err := pgvector.QuoteIdent(indexName)
	if err != nil {
		return pgvector.Failf("%s", err)
	}

	result := map[string]interface{}{
		"index_name":      indexName,
		"schema":          auth.Schema,
		"table":           auth.Table,
		"column":          vectorColumn,
		"index_type":      indexType,
		"distance_metric": metricName,
		"ops_class":       metric.OpsClass,
		"dimensions":      dimensions,
	}

	existingOps, existingTable, err := lookupIndex(ctx, db, auth.Schema, indexName)
	if err != nil {
		return pgvector.Fail(auth, err)
	}
	if existingOps != "" {
		switch {
		case existingTable != auth.Table:
			return pgvector.Failf(
				"an index called %q already exists in schema %s, but it belongs to the table %q — drop or rename it "+
					"before indexing %s",
				indexName, auth.Schema, existingTable, auth.Table)

		// Same table, same column, same method, different metric: IF NOT EXISTS
		// would sail straight past this and leave the operator with an index
		// their searches can never use.
		case existingOps != metric.OpsClass:
			return pgvector.Failf(
				"%s.%s already has an index called %q, but it was built for %s distance, so %s searches still can't "+
					"use it. Either set Distance Metric to %s, or drop the old index and re-run this step: "+
					"DROP INDEX %s.%s;",
				auth.Schema, auth.Table, indexName, metricLabel(existingOps), metricLabel(metric.OpsClass),
				metricLabel(existingOps), qSchema, qIndex)
		}

		result["created"] = false
		return pgvector.OK(map[string]interface{}{
			"index_name": indexName,
			"created":    false,
			"result":     result,
		}, fmt.Sprintf(
			"%s.%s already has the %s index %s over %s, tuned for %s search. Nothing to do.",
			auth.Schema, auth.Table, indexLabels[indexType], indexName, vectorColumn,
			metricLabel(metric.OpsClass))), nil
	}

	// An IVFFlat index is a clustering of the rows that exist when it is built,
	// so one built on an empty table stays useless no matter what is inserted
	// afterwards. EXISTS rather than count(*) because this only needs to know
	// empty-or-not, and a full count on a large table would eat the time budget
	// the index build itself needs.
	populated := true
	if indexType == "ivfflat" {
		if err := db.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM "+qRel+")").Scan(&populated); err != nil {
			return pgvector.Fail(auth, err)
		}
	}

	stmt := "CREATE INDEX "
	if concurrently {
		// CREATE INDEX CONCURRENTLY cannot run inside a transaction block, which
		// is why this statement is executed on its own and never wrapped in one.
		stmt += "CONCURRENTLY "
	}
	stmt += "IF NOT EXISTS " + qIndex + " ON " + qRel + " USING " + indexType +
		" (" + qVector + " " + metric.OpsClass + ")"
	switch indexType {
	case "hnsw":
		stmt += " WITH (m = " + strconv.Itoa(links) + ", ef_construction = " + strconv.Itoa(effort) + ")"
	case "ivfflat":
		stmt += " WITH (lists = " + strconv.Itoa(lists) + ")"
	}

	if _, err := db.ExecContext(ctx, stmt); err != nil {
		if cancelled(ctx, err) {
			return pgvector.Failf(
				"The index was still building when this step ran out of time, so Postgres cancelled it. Indexing a "+
					"large table takes longer than a workflow step can wait for — run it on the database itself "+
					"instead:\n  %s;\nPostgres may also have left a half-built index behind, which searches ignore. "+
					"Clear that first with: DROP INDEX IF EXISTS %s.%s;",
				stmt, qSchema, qIndex)
		}
		return pgvector.Fail(auth, err)
	}

	// The planner will not choose an index it has no statistics for, so skipping
	// this is the difference between building an index and using one.
	var warnings []string
	analyzed := false
	if analyzeAfter {
		if _, err := db.ExecContext(ctx, "ANALYZE "+qRel); err != nil {
			warnings = append(warnings, "The index is built, but updating the table statistics afterwards failed: "+
				pgvector.Humanise(auth, err)+". Searches may not use the new index until someone runs ANALYZE "+
				auth.Schema+"."+auth.Table+".")
		} else {
			analyzed = true
		}
	}
	if indexType == "ivfflat" && !populated {
		warnings = append(warnings, fmt.Sprintf(
			"Warning: %s.%s has no rows in it. An IVFFlat index groups the rows that exist when it is built, so one "+
				"built on an empty table stays useless — add your documents, then drop and rebuild this index.",
			auth.Schema, auth.Table))
	}

	parameters := map[string]interface{}{}
	switch indexType {
	case "hnsw":
		parameters["m"] = links
		parameters["ef_construction"] = effort
	case "ivfflat":
		parameters["lists"] = lists
	}
	result["created"] = true
	result["concurrently"] = concurrently
	result["analyzed"] = analyzed
	result["parameters"] = parameters

	summary := fmt.Sprintf(
		"Created the %s index %s on %s.%s over %s, tuned for %s search.",
		indexLabels[indexType], indexName, auth.Schema, auth.Table, vectorColumn, metricLabel(metric.OpsClass))
	for _, w := range warnings {
		summary += " " + w
	}

	return pgvector.OK(map[string]interface{}{
		"index_name": indexName,
		"created":    true,
		"result":     result,
	}, summary), nil
}

// lookupIndex reads back the operator class and parent table of an existing
// index, returning "" when there is no index of that name.
//
// The index name carries the table, the column and the method, but not the
// metric — so a second run with a different Distance Metric produces the same
// name, IF NOT EXISTS reports nothing, and the operator is left with an index
// their searches will never touch. indclass is the only place that mismatch
// shows up.
func lookupIndex(ctx context.Context, db *sql.DB, schema, name string) (opsClass, table string, err error) {
	err = db.QueryRowContext(ctx, `
		SELECT o.opcname, t.relname
		  FROM pg_index     i
		  JOIN pg_class     c ON c.oid = i.indexrelid
		  JOIN pg_class     t ON t.oid = i.indrelid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		  JOIN pg_opclass   o ON o.oid = i.indclass[0]
		 WHERE n.nspname = $1
		   AND c.relname = $2`, schema, name).Scan(&opsClass, &table)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", err
	}
	return opsClass, table, nil
}

// cancelled reports whether a build was killed by a timeout rather than failing
// on its own merits — the server-side statement_timeout fires first, so this is
// usually a pq error rather than a dead context.
//
// Worth telling apart, because the standard advice for a cancelled statement is
// "add an index", which is no help at all inside the Create Index step.
func cancelled(ctx context.Context, err error) bool {
	if ctx.Err() != nil {
		return true
	}
	var pe *pq.Error
	return errors.As(err, &pe) && pe.Code == "57014"
}

// metricLabel names an operator class the way the Distance Metric dropdown does.
func metricLabel(opsClass string) string {
	if name, ok := opsClassMetric[opsClass]; ok {
		return name
	}
	return opsClass
}
