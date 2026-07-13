// Package pgvector provides the shared Postgres client behind every
// Vector Database ▸ pgvector action.
//
// pgvector is an extension to Postgres, not a hosted service, so unlike the
// REST providers there is no base URL and no bearer token: an action dials the
// customer's database over the Postgres wire protocol, does one piece of work,
// and disconnects. Connections are per-execution and never pooled — the same
// lifecycle the SQL and Redis actions use.
//
// Four properties of the extension drive the shape of this file.
//
//   - A `vector` column is dimensioned (vector(1536)). Postgres refuses, hard,
//     to compare vectors of different lengths. That is a *good* failure — it
//     beats silently returning nonsense — but the raw driver error is
//     "different vector dimensions 1024 and 1536", which means nothing to a
//     front-of-house operator. Every query that carries a vector therefore
//     preflights the table's declared dimension and explains the mismatch in
//     terms of the embedding model that caused it. See TableDimension and
//     Humanise.
//
//   - There is no canonical schema. LangChain (and therefore n8n) writes
//     id/text/metadata/embedding; a hand-rolled table is as likely to be
//     content/embedding. The node serves both by auto-detecting the columns
//     from the catalog when the operator leaves them blank, which is what makes
//     "point it at your existing table and it just works" possible. See
//     ResolveColumns.
//
//   - Table and column names cannot be bound as $1 parameters — they have to be
//     interpolated into the SQL text. That is the one genuine injection surface
//     in this package, and QuoteIdent/QuoteRelation are the only sanctioned way
//     across it. Every other user value is bound.
//
//   - Binding even one parameter moves lib/pq off the simple query protocol,
//     which is what makes "SELECT 1; DROP TABLE users" possible in the legacy
//     SQL node. Every statement here either binds a parameter or consists
//     solely of validated identifiers and range-checked integers.
package pgvector

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	core "flomation.app/automate/executor"
	"github.com/lib/pq"
)

const (
	// ConnectTimeoutSeconds bounds the TCP dial + startup handshake. Without it
	// a black-holed host hangs until the OS TCP timeout (~2 minutes).
	ConnectTimeoutSeconds = 5

	// StatementTimeout is applied server-side, per session, immediately after
	// connect. Connections here are never pooled, so a session-level SET cannot
	// leak into anyone else's query.
	StatementTimeout = "30s"

	// QueryTimeout bounds a single statement client-side. lib/pq's watchCancel
	// turns a context deadline into a real Postgres CancelRequest on a second
	// connection, so this genuinely kills the server-side query rather than
	// just abandoning the caller.
	QueryTimeout = 60 * time.Second

	// MaxRows caps any result-set scan loop. The legacy SQL node accumulates
	// every row unbounded; a SELECT * over a million-row table of 1536-float
	// vectors is enough to OOM the executor.
	MaxRows = 1000

	// MaxAllPages caps document_list's return_all so a runaway table cannot pin
	// an executor slot forever.
	MaxAllPages = 20

	// MaxBatchDocuments caps a single insert/upsert batch.
	MaxBatchDocuments = 1000

	// summaryPreview caps how much document text is echoed into tool_result,
	// which is a human-facing summary, not a data channel.
	summaryPreview = 200
)

// Auth is the database connection config shared by every action. It is
// assembled from the connection block that each action re-declares in its
// Inputs (the manifest generator AST-parses those literals, so they cannot be
// factored out into a shared var).
type Auth struct {
	Host     string
	Port     int64
	Database string
	Username string
	Password string
	SSLMode  string
	Schema   string
	Table    string
}

// AuthInputs documents the canonical connection block. Action packages
// re-declare their own literal Inputs arrays (the manifest generator AST-parses
// those and cannot follow a variable reference), so this exists for reference
// and to keep the labels and placeholders in one place when they change.
// actions/infrastructure/inputs_drift_test.go asserts the copies stay in step.
var AuthInputs = []core.Connection{
	{Name: "host", Type: core.ConnectionTypeString, Label: "Database Host", Placeholder: "db.example.com or 192.168.1.20 — hostname or IP, no scheme", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "5432"},
	{Name: "database", Type: core.ConnectionTypeString, Label: "Database", Placeholder: "vectordb", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "postgres", Required: true},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Placeholder: "Database password", Required: true},
	{Name: "ssl_mode", Type: core.ConnectionTypeString, Label: "SSL Mode", Placeholder: "disable", Options: SSLModeOptions},
	{Name: "schema", Type: core.ConnectionTypeString, Label: "Schema", Placeholder: "public"},
}

// SSLModeOptions are libpq's six sslmode values, labelled the way an operator
// thinks about them rather than the way the manual writes them.
var SSLModeOptions = []core.ConnectionOption{
	{Name: "Disable — no encryption", Value: "disable"},
	{Name: "Allow", Value: "allow"},
	{Name: "Prefer — encrypt if the server offers it", Value: "prefer"},
	{Name: "Require — encrypt, but don't verify the certificate", Value: "require"},
	{Name: "Verify CA — encrypt and check the certificate authority", Value: "verify-ca"},
	{Name: "Verify Full — encrypt and check the hostname too", Value: "verify-full"},
}

// DistanceMetricOptions are the three distance functions pgvector indexes can
// serve. Cosine is the right default for text embeddings and is what every
// mainstream embedding model is trained for.
var DistanceMetricOptions = []core.ConnectionOption{
	{Name: "Cosine — best for text embeddings", Value: "cosine"},
	{Name: "Inner Product", Value: "inner_product"},
	{Name: "Euclidean (L2)", Value: "euclidean"},
}

// metric holds everything that must agree between a query and its index. Get
// the ops class wrong and the index is silently never used — the query still
// returns the right answer, just via a sequential scan over every row.
type metric struct {
	Operator string // the distance operator used in ORDER BY
	OpsClass string // the index operator class that operator can use
}

var metrics = map[string]metric{
	"cosine":        {Operator: "<=>", OpsClass: "vector_cosine_ops"},
	"inner_product": {Operator: "<#>", OpsClass: "vector_ip_ops"},
	"euclidean":     {Operator: "<->", OpsClass: "vector_l2_ops"},
}

// Metric resolves a distance_metric input, defaulting to cosine.
func Metric(name string) (metric, error) {
	if strings.TrimSpace(name) == "" {
		return metrics["cosine"], nil
	}
	m, ok := metrics[strings.TrimSpace(name)]
	if !ok {
		return metric{}, fmt.Errorf("unknown distance metric %q — use cosine, inner_product or euclidean", name)
	}
	return m, nil
}

// Similarity converts a raw pgvector distance into a "higher is better" score.
//
// This exists because the raw operator output means different things per metric
// (cosine distance is 0..2, L2 is 0..∞, and pgvector negates inner product so
// that "closer" is still "smaller"), and an operator setting a minimum
// relevance threshold should not have to know which. Actions emit BOTH the raw
// distance and this score, and min_score always filters on the score.
func Similarity(metricName string, distance float64) float64 {
	switch metricName {
	case "euclidean":
		return 1 / (1 + distance)
	case "inner_product":
		// pgvector's <#> returns the negative inner product.
		return -distance
	default: // cosine: distance runs 0 (identical) .. 2 (opposite)
		return (2 - distance) / 2
	}
}

// ---------------------------------------------------------------------------
// Identifiers
// ---------------------------------------------------------------------------

// identRe is deliberately stricter than Postgres itself. Postgres will accept
// almost anything inside double quotes; we accept only the shape a real table
// or column has, so that a value which has no business being an identifier is
// rejected at the door rather than being quoted and passed through.
//
// NAMEDATALEN is 64, giving 63 usable bytes.
var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_$]{0,62}$`)

// QuoteIdent validates an identifier and returns it safely quoted.
//
// Both halves matter. pq.QuoteIdentifier is injection-safe on its own — it
// doubles any embedded quote — but it truncates silently at a NUL byte, so it
// is not sufficient by itself. The regex rejects NUL, whitespace, quotes, dots
// and every other smuggling vector before the quoting ever runs.
//
// Quoting makes the identifier case-sensitive, which is correct: the dropdowns
// return the catalog's exact spelling. It does mean a user who free-types
// "MyTable" for a table actually named mytable gets a relation-does-not-exist
// error — Humanise explains that one.
func QuoteIdent(name string) (string, error) {
	if !identRe.MatchString(name) {
		return "", fmt.Errorf(
			"%q is not a valid table or column name — use letters, numbers and underscores, starting with a letter", name)
	}
	return pq.QuoteIdentifier(name), nil
}

// QuoteRelation quotes a schema-qualified relation, validating each part
// separately.
//
// Never pass "public.docs" to QuoteIdent: that yields the single quoted
// identifier "public.docs", which is a table whose name literally contains a
// dot — not the table docs in schema public.
func QuoteRelation(schema, table string) (string, error) {
	if strings.TrimSpace(schema) == "" {
		schema = "public"
	}
	qs, err := QuoteIdent(schema)
	if err != nil {
		return "", err
	}
	qt, err := QuoteIdent(table)
	if err != nil {
		return "", err
	}
	return qs + "." + qt, nil
}

// ---------------------------------------------------------------------------
// Connection
// ---------------------------------------------------------------------------

// GetAuth reads the connection block out of an action's inputs.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	a := Auth{
		Host:     OptionalString(core.FindConnection("host", inputs)),
		Database: OptionalString(core.FindConnection("database", inputs)),
		Username: OptionalString(core.FindConnection("username", inputs)),
		Password: OptionalString(core.FindConnection("password", inputs)),
		SSLMode:  OptionalString(core.FindConnection("ssl_mode", inputs)),
		Schema:   OptionalString(core.FindConnection("schema", inputs)),
		Table:    OptionalString(core.FindConnection("table", inputs)),
	}

	a.Port = 5432
	if p := core.FindConnection("port", inputs); p != nil {
		if n := p.Number(); n != nil && *n > 0 {
			a.Port = *n
		}
	}
	if a.Schema == "" {
		a.Schema = "public"
	}
	if a.SSLMode == "" {
		a.SSLMode = "disable"
	}

	switch {
	case a.Host == "":
		return a, errors.New("Database Host is required")
	case a.Database == "":
		return a, errors.New("Database is required")
	case a.Username == "":
		return a, errors.New("Username is required")
	}
	if a.Port < 1 || a.Port > 65535 {
		return a, fmt.Errorf("Port %d is not a valid port number", a.Port)
	}
	if _, ok := validSSLModes[a.SSLMode]; !ok {
		return a, fmt.Errorf("%q is not a valid SSL mode", a.SSLMode)
	}
	return a, nil
}

var validSSLModes = map[string]struct{}{
	"disable": {}, "allow": {}, "prefer": {},
	"require": {}, "verify-ca": {}, "verify-full": {},
}

// OptionalString reads a string/secret input, returning "" when it is unset.
//
// The nil-check is load-bearing for ConnectionTypeSecret. Connection.String()
// special-cases the text-ish types and otherwise falls through to a
// fmt.Sprintf("%v", …), so a secret input that was never filled in stringifies
// to the literal text "<nil>" — which would then be sent to Postgres as the
// password.
func OptionalString(c *core.Connection) string {
	if c == nil || c.Value == nil {
		return ""
	}
	s := c.String()
	if s == nil {
		return ""
	}
	return strings.TrimSpace(*s)
}

// OptionalBool reads a boolean input, falling back to def when it is unset.
func OptionalBool(c *core.Connection, def bool) bool {
	if c == nil {
		return def
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	return def
}

// OptionalInt reads an integer input, falling back to def when it is unset.
func OptionalInt(c *core.Connection, def int) int {
	if c == nil {
		return def
	}
	if n := c.Number(); n != nil {
		return int(*n)
	}
	return def
}

// DSN builds the libpq connection URL.
//
// Built with net/url rather than fmt.Sprintf (which is what the legacy SQL node
// uses) so that a password containing '@' or '/' cannot restructure the URL,
// and so a host cannot smuggle extra libpq connection parameters.
func (a Auth) DSN() string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(a.Username, a.Password),
		Host:   net.JoinHostPort(a.Host, strconv.FormatInt(a.Port, 10)),
		Path:   "/" + a.Database,
	}
	q := url.Values{}
	q.Set("sslmode", a.SSLMode)
	q.Set("connect_timeout", strconv.Itoa(ConnectTimeoutSeconds))
	u.RawQuery = q.Encode()
	return u.String()
}

// OpenConn dials the database and returns a live, bounded connection.
//
// No SSRF allowlist here, deliberately. Reaching an arbitrary host:port IS the
// job of a database node — customers' databases live on private networks — and
// every other protocol node in the executor (sql/*, nosql/*, mqtt, kubernetes,
// filetransfer) takes the same position. The api's options proxy is a different
// trust boundary and IS hardened; see api/internal/http/pgvector_options.go.
func OpenConn(ctx context.Context, a Auth) (*sql.DB, error) {
	db, err := sql.Open("postgres", a.DSN())
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(2)
	db.SetMaxIdleConns(0)
	db.SetConnMaxLifetime(2 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, err
	}

	// Session-scoped, and the session dies with this action.
	if _, err := db.ExecContext(ctx, "SET statement_timeout = '"+StatementTimeout+"'"); err != nil {
		db.Close()
		return nil, err
	}
	return db, nil
}

// Context derives the per-action deadline from the flow's context, so that
// cancelling a run actually cancels the query rather than orphaning it.
func Context(flow *core.Flow) (context.Context, context.CancelFunc) {
	parent := context.Background()
	if flow != nil {
		if c := flow.GoContext(); c != nil {
			parent = c
		}
	}
	return context.WithTimeout(parent, QueryTimeout)
}

// ---------------------------------------------------------------------------
// Column resolution
// ---------------------------------------------------------------------------

// ColumnInputs are the four column names as the operator left them — any of
// which may be empty, meaning "work it out for me".
type ColumnInputs struct {
	ID       string
	Content  string
	Metadata string
	Vector   string
}

// ColumnSet is the resolved, validated, quoted result.
type ColumnSet struct {
	ID       string // raw name
	Content  string
	Metadata string // "" when the table has no metadata column at all
	Vector   string

	QID       string // pre-quoted, ready to interpolate
	QContent  string
	QMetadata string // "" when Metadata is ""
	QVector   string
}

// HasMetadata reports whether the table can store metadata at all. A table
// without a jsonb column is perfectly usable for search — it just cannot
// filter — so this is a soft capability check, not an error.
func (c ColumnSet) HasMetadata() bool { return c.Metadata != "" }

// column candidate lists, in preference order. The first entry of each is the
// LangChain/n8n default, so a table built by n8n resolves on the first probe.
var (
	contentCandidates = []string{"text", "content", "document", "body", "chunk", "page_content"}
	metaCandidates    = []string{"metadata", "meta", "cmetadata"}
)

var textishTypes = map[string]struct{}{
	"text": {}, "varchar": {}, "bpchar": {}, "citext": {},
}

var jsonTypes = map[string]struct{}{"jsonb": {}, "json": {}}

type columnInfo struct {
	Name      string
	UDTName   string
	Dimension int // vector columns only; -1 when unbounded
}

// ResolveColumns turns whatever the operator supplied into a validated ColumnSet.
//
// Priority is: an explicit name wins; anything left blank is auto-detected from
// the catalog. Auto-detect is the reason a brand-new user can point this node
// at a table someone else built and have it work without knowing what a vector
// column is.
func ResolveColumns(ctx context.Context, db *sql.DB, schema, table string, in ColumnInputs) (ColumnSet, error) {
	cols, err := describeColumns(ctx, db, schema, table)
	if err != nil {
		return ColumnSet{}, err
	}
	if len(cols) == 0 {
		return ColumnSet{}, fmt.Errorf(
			"table %s.%s doesn't exist, or this user can't see it", schema, table)
	}

	byName := make(map[string]columnInfo, len(cols))
	names := make([]string, 0, len(cols))
	for _, c := range cols {
		byName[c.Name] = c
		names = append(names, c.Name)
	}

	// explicit(): validate a name the operator typed or picked.
	explicit := func(field, name string) (string, error) {
		if _, ok := byName[name]; !ok {
			return "", fmt.Errorf(
				"the %s column %q doesn't exist on %s.%s — available columns are: %s",
				field, name, schema, table, strings.Join(names, ", "))
		}
		return name, nil
	}

	out := ColumnSet{}

	// --- vector column: the one column we cannot do without ---
	switch {
	case in.Vector != "":
		v, err := explicit("embedding", in.Vector)
		if err != nil {
			return ColumnSet{}, err
		}
		if byName[v].UDTName != "vector" {
			return ColumnSet{}, fmt.Errorf(
				"column %q on %s.%s is a %s, not a vector — pick the column that holds the embeddings",
				v, schema, table, byName[v].UDTName)
		}
		out.Vector = v
	default:
		var found []string
		for _, c := range cols {
			if c.UDTName == "vector" {
				found = append(found, c.Name)
			}
		}
		switch len(found) {
		case 0:
			return ColumnSet{}, fmt.Errorf(
				"%s.%s has no vector column, so it isn't a pgvector table yet — run Create Table first, or point this step at a table that has embeddings in it",
				schema, table)
		case 1:
			out.Vector = found[0]
		default:
			return ColumnSet{}, fmt.Errorf(
				"%s.%s has more than one vector column (%s) — set Embedding Column to say which one to use",
				schema, table, strings.Join(found, ", "))
		}
	}

	// --- content column ---
	if in.Content != "" {
		c, err := explicit("content", in.Content)
		if err != nil {
			return ColumnSet{}, err
		}
		out.Content = c
	} else {
		for _, cand := range contentCandidates {
			if c, ok := byName[cand]; ok {
				if _, textish := textishTypes[c.UDTName]; textish {
					out.Content = cand
					break
				}
			}
		}
		if out.Content == "" {
			return ColumnSet{}, fmt.Errorf(
				"couldn't work out which column on %s.%s holds the document text — set Content Column. Available columns: %s",
				schema, table, strings.Join(names, ", "))
		}
	}

	// --- metadata column (optional: a table without one still searches fine) ---
	if in.Metadata != "" {
		m, err := explicit("metadata", in.Metadata)
		if err != nil {
			return ColumnSet{}, err
		}
		if _, ok := jsonTypes[byName[m].UDTName]; !ok {
			return ColumnSet{}, fmt.Errorf(
				"column %q on %s.%s is a %s, not jsonb — metadata has to live in a jsonb column",
				m, schema, table, byName[m].UDTName)
		}
		out.Metadata = m
	} else {
		for _, cand := range metaCandidates {
			if c, ok := byName[cand]; ok {
				if _, isJSON := jsonTypes[c.UDTName]; isJSON {
					out.Metadata = cand
					break
				}
			}
		}
	}

	// --- id column ---
	if in.ID != "" {
		i, err := explicit("ID", in.ID)
		if err != nil {
			return ColumnSet{}, err
		}
		out.ID = i
	} else {
		pk, err := primaryKeyColumn(ctx, db, schema, table)
		if err != nil {
			return ColumnSet{}, err
		}
		switch {
		case pk != "":
			out.ID = pk
		default:
			if _, ok := byName["id"]; ok {
				out.ID = "id"
			} else {
				return ColumnSet{}, fmt.Errorf(
					"%s.%s has no primary key and no \"id\" column — set ID Column. Available columns: %s",
					schema, table, strings.Join(names, ", "))
			}
		}
	}

	// Quote everything exactly once, here, so no caller has to remember to.
	var err2 error
	if out.QID, err2 = QuoteIdent(out.ID); err2 != nil {
		return ColumnSet{}, err2
	}
	if out.QContent, err2 = QuoteIdent(out.Content); err2 != nil {
		return ColumnSet{}, err2
	}
	if out.QVector, err2 = QuoteIdent(out.Vector); err2 != nil {
		return ColumnSet{}, err2
	}
	if out.Metadata != "" {
		if out.QMetadata, err2 = QuoteIdent(out.Metadata); err2 != nil {
			return ColumnSet{}, err2
		}
	}
	return out, nil
}

// describeColumns reads the catalog. format_type is what carries the vector's
// declared dimension — information_schema reports it only as "USER-DEFINED".
func describeColumns(ctx context.Context, db *sql.DB, schema, table string) ([]columnInfo, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT a.attname,
		       t.typname,
		       format_type(a.atttypid, a.atttypmod) AS formatted
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
		return nil, err
	}
	defer rows.Close()

	var out []columnInfo
	for rows.Next() {
		var ci columnInfo
		var formatted string
		if err := rows.Scan(&ci.Name, &ci.UDTName, &formatted); err != nil {
			return nil, err
		}
		ci.Dimension = parseVectorDimension(ci.UDTName, formatted)
		out = append(out, ci)
	}
	return out, rows.Err()
}

// vectorTypeRe pulls N out of "vector(1536)". A bare "vector" is an unbounded
// column — which is exactly what n8n creates, and why an n8n-built table can
// never be given an ANN index.
var vectorTypeRe = regexp.MustCompile(`^vector\((\d+)\)$`)

func parseVectorDimension(udt, formatted string) int {
	if udt != "vector" {
		return 0
	}
	if m := vectorTypeRe.FindStringSubmatch(formatted); m != nil {
		if n, err := strconv.Atoi(m[1]); err == nil {
			return n
		}
	}
	return -1 // declared as bare `vector`: any dimension accepted
}

func primaryKeyColumn(ctx context.Context, db *sql.DB, schema, table string) (string, error) {
	var name string
	err := db.QueryRowContext(ctx, `
		SELECT a.attname
		  FROM pg_index     i
		  JOIN pg_class     c ON c.oid = i.indrelid
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		  JOIN pg_attribute a ON a.attrelid = c.oid AND a.attnum = ANY(i.indkey)
		 WHERE n.nspname = $1
		   AND c.relname = $2
		   AND i.indisprimary
		 ORDER BY a.attnum
		 LIMIT 1`, schema, table).Scan(&name)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return name, nil
}

// TableDimension returns the declared dimension of a vector column: N for
// vector(N), -1 for an unbounded `vector`, 0 when the column is absent.
func TableDimension(ctx context.Context, db *sql.DB, schema, table, column string) (int, error) {
	cols, err := describeColumns(ctx, db, schema, table)
	if err != nil {
		return 0, err
	}
	for _, c := range cols {
		if c.Name == column {
			return c.Dimension, nil
		}
	}
	return 0, nil
}

// CheckDimension preflights a vector against the table's declared dimension.
//
// Postgres would catch this anyway — that is the whole point of a dimensioned
// column, and the error it raises is a hard one rather than a silently wrong
// answer. But it raises it in terms of two integers, and the operator needs it
// in terms of the thing they actually got wrong: they pointed a 1536-dimension
// OpenAI embedding at a table built for 1024-dimension Titan vectors. Catching
// it before the query lets us say so.
func CheckDimension(declared int, vec []float32, table string) error {
	if declared <= 0 || len(vec) == 0 || declared == len(vec) {
		return nil
	}
	return fmt.Errorf(
		"this embedding has %d dimensions but %s stores %d-dimension vectors — they have to match. "+
			"That usually means the embedding model here isn't the one the table was built for "+
			"(OpenAI text-embedding-3-small is 1536, Bedrock Titan v2 is 1024). "+
			"Either switch the model, or point this step at a table with %d-dimension vectors",
		len(vec), table, declared, len(vec))
}

// ---------------------------------------------------------------------------
// Errors
// ---------------------------------------------------------------------------

// Redact removes credentials from a message before it is shown to anyone. The
// SQL nodes do not do this, and lib/pq's connection errors can echo the DSN.
func Redact(a Auth, msg string) string {
	if a.Password != "" {
		msg = strings.ReplaceAll(msg, a.Password, "********")
	}
	return msg
}

// Humanise turns a driver error into something a front-of-house operator can
// act on. Everything else falls through unchanged — inventing a friendly
// message for an error we did not anticipate would only hide it.
func Humanise(a Auth, err error) string {
	if err == nil {
		return ""
	}
	msg := Redact(a, err.Error())

	var pe *pq.Error
	if errors.As(err, &pe) {
		switch {
		case pe.Code == "22000" && strings.Contains(pe.Message, "different vector dimensions"):
			return fmt.Sprintf(
				"%s. The embedding you supplied is a different size from the ones this table stores — "+
					"check that the embedding model matches the table (OpenAI text-embedding-3-small is 1536, "+
					"Bedrock Titan v2 is 1024)", pe.Message)

		case pe.Code == "42P01": // undefined_table
			return fmt.Sprintf(
				"%s. Table names are case-sensitive here, so \"MyTable\" and \"mytable\" are different tables — "+
					"pick the table from the dropdown to be sure", pe.Message)

		case pe.Code == "42704" && strings.Contains(strings.ToLower(pe.Message), "vector"):
			return "The pgvector extension isn't installed on this database. Tick \"Create the extension\" on a " +
				"Create Table step, or ask your DBA to run: CREATE EXTENSION vector;"

		case pe.Code == "42501": // insufficient_privilege
			return fmt.Sprintf("%s. The database user %q doesn't have permission for that", pe.Message, a.Username)

		case pe.Code == "28P01" || pe.Code == "28000": // invalid_password / invalid_authorization
			return fmt.Sprintf("Postgres rejected the login for user %q — check the username and password", a.Username)

		case pe.Code == "3D000": // invalid_catalog_name
			return fmt.Sprintf("The database %q doesn't exist on this server", a.Database)

		case pe.Code == "57014": // query_canceled
			return fmt.Sprintf(
				"The query took longer than %s and was cancelled. Add an index (see the Create Index step) "+
					"or narrow the search", StatementTimeout)

		case pe.Code == "23505": // unique_violation
			return fmt.Sprintf(
				"%s. A document with that ID already exists — use the Upsert Document step to overwrite it "+
					"instead of Insert", pe.Message)
		}
		return pe.Message
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Sprintf("The database didn't respond within %s", QueryTimeout)
	}

	var ne net.Error
	if errors.As(err, &ne) || strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host") {
		return fmt.Sprintf("Couldn't reach the database at %s:%d — %s", a.Host, a.Port, msg)
	}
	return msg
}

// ---------------------------------------------------------------------------
// Result shapers
// ---------------------------------------------------------------------------

// ErrorResult is the standard soft-failure output map (returned with a nil
// error so the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// Fail is the shorthand every Execute uses: humanise, redact, and shape.
func Fail(a Auth, err error) (map[string]interface{}, error) {
	return ErrorResult(Humanise(a, err)), nil
}

// Failf is Fail for errors we raise ourselves, which are already human.
func Failf(format string, args ...interface{}) (map[string]interface{}, error) {
	return ErrorResult(fmt.Sprintf(format, args...)), nil
}

// OK merges the standard success outputs into an action's own.
func OK(fields map[string]interface{}, summary string) map[string]interface{} {
	if fields == nil {
		fields = map[string]interface{}{}
	}
	fields["tool_result"] = summary
	fields["success"] = true
	fields["error"] = ""
	return fields
}

// Preview truncates document text for a human-facing summary line.
//
// The cut is backed off to a rune boundary. summaryPreview is a byte budget,
// and slicing a string at an arbitrary byte index splits any multi-byte rune
// that straddles it — which for a document containing é, €, or an emoji yields
// invalid UTF-8. That matters more than it looks: this text lands in
// tool_result, which is both what the operator reads in the UI and what is
// handed to an LLM when the step runs as an agent tool. encoding/json quietly
// rewrites the broken bytes to U+FFFD rather than erroring, so the only symptom
// is mojibake.
func Preview(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= summaryPreview {
		return s
	}
	cut := summaryPreview
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut] + "…"
}

// Clamp bounds a limit input.
func Clamp(n, def, max int) int {
	if n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}
