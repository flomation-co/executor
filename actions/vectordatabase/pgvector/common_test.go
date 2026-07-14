package pgvector

import (
	"context"
	"database/sql"
	"net/url"
	"strings"
	"testing"

	"unicode/utf8"

	core "flomation.app/automate/executor"
	"github.com/DATA-DOG/go-sqlmock"
	"github.com/lib/pq"
	. "github.com/onsi/gomega"
)

// ---------------------------------------------------------------------------
// Identifiers — the one genuine injection surface in the package
// ---------------------------------------------------------------------------

func TestQuoteIdent_Accepts(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		in   string
		want string
	}{
		{"docs", `"docs"`},
		{"documents", `"documents"`},
		{"_private", `"_private"`},
		{"MyTable", `"MyTable"`}, // quoting keeps the catalog's exact spelling
		{"col$1", `"col$1"`},
		{"a1_b2", `"a1_b2"`},
		{strings.Repeat("a", 63), `"` + strings.Repeat("a", 63) + `"`}, // NAMEDATALEN-1
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			RegisterTestingT(t)
			got, err := QuoteIdent(tt.in)
			Expect(err).ToNot(HaveOccurred())
			Expect(got).To(Equal(tt.want))
		})
	}
}

// Table and column names cannot be bound as $1 — they have to be interpolated
// into the SQL text. This is the gate that stands in front of that, so every
// smuggling vector gets its own case.
func TestQuoteIdent_Rejects(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name string
		in   string
	}{
		{"classic injection", `docs"; DROP TABLE x --`},
		{"quote only", `do"cs`},
		{"single quote", `do'cs`},
		{"semicolon", "docs;"},
		{"NUL byte (pq.QuoteIdentifier truncates at one)", "docs\x00; DROP TABLE x"},
		{"empty", ""},
		{"whitespace", "my docs"},
		{"leading digit", "1docs"},
		{"leading dollar", "$docs"},
		{"too long (64)", strings.Repeat("a", 64)},
		{"schema-qualified dot", "public.docs"},
		{"trailing newline", "docs\n"},
		{"hyphen", "my-docs"},
		{"comment", "docs--"},
		{"parenthesis", "docs()"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)
			_, err := QuoteIdent(tt.in)
			Expect(err).To(HaveOccurred(), "QuoteIdent(%q) must be rejected", tt.in)
			Expect(err.Error()).To(ContainSubstring("not a valid table or column name"))
		})
	}
}

// "public.docs" quoted as a single identifier is "public.docs" — a table whose
// name literally contains a dot, which is a DIFFERENT table from docs in schema
// public. Rejecting it at QuoteIdent and splitting at QuoteRelation is the only
// way to get this right.
func TestQuoteRelation(t *testing.T) {
	RegisterTestingT(t)

	got, err := QuoteRelation("public", "docs")
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(Equal(`"public"."docs"`))

	// A blank schema defaults to public.
	got, err = QuoteRelation("", "docs")
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(Equal(`"public"."docs"`))

	got, err = QuoteRelation("  ", "docs")
	Expect(err).ToNot(HaveOccurred())
	Expect(got).To(Equal(`"public"."docs"`))

	// The trap this function exists to avoid.
	_, err = QuoteIdent("public.docs")
	Expect(err).To(HaveOccurred(),
		`QuoteIdent("public.docs") must be rejected — quoting it whole targets a table whose name contains a dot`)

	// Both halves are validated.
	_, err = QuoteRelation(`pub"lic`, "docs")
	Expect(err).To(HaveOccurred())
	_, err = QuoteRelation("public", `do"cs`)
	Expect(err).To(HaveOccurred())
}

// ---------------------------------------------------------------------------
// GetAuth
// ---------------------------------------------------------------------------

func conn(name, typ string, val any) *core.Connection {
	return &core.Connection{Name: name, Type: typ, Value: val}
}

func fullAuthInputs() []*core.Connection {
	return []*core.Connection{
		conn("host", core.ConnectionTypeString, "db.example.com"),
		conn("database", core.ConnectionTypeString, "vectordb"),
		conn("username", core.ConnectionTypeString, "postgres"),
		conn("password", core.ConnectionTypeSecret, "hunter2"),
	}
}

func TestGetAuth_Defaults(t *testing.T) {
	RegisterTestingT(t)

	a, err := GetAuth(fullAuthInputs())
	Expect(err).ToNot(HaveOccurred())
	Expect(a.Host).To(Equal("db.example.com"))
	Expect(a.Database).To(Equal("vectordb"))
	Expect(a.Username).To(Equal("postgres"))
	Expect(a.Password).To(Equal("hunter2"))
	Expect(a.Port).To(Equal(int64(5432)))
	Expect(a.Schema).To(Equal("public"))
	Expect(a.SSLMode).To(Equal("disable"))
}

func TestGetAuth_ExplicitValues(t *testing.T) {
	RegisterTestingT(t)

	inputs := append(fullAuthInputs(),
		conn("port", core.ConnectionTypeInteger, 6432),
		conn("schema", core.ConnectionTypeString, "rag"),
		conn("ssl_mode", core.ConnectionTypeString, "verify-full"),
		conn("table", core.ConnectionTypeString, "docs"),
	)
	a, err := GetAuth(inputs)
	Expect(err).ToNot(HaveOccurred())
	Expect(a.Port).To(Equal(int64(6432)))
	Expect(a.Schema).To(Equal("rag"))
	Expect(a.SSLMode).To(Equal("verify-full"))
	Expect(a.Table).To(Equal("docs"))
}

func TestGetAuth_Errors(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name    string
		mutate  func([]*core.Connection) []*core.Connection
		wantErr string
	}{
		{
			name: "missing host",
			mutate: func(in []*core.Connection) []*core.Connection {
				in[0].Value = ""
				return in
			},
			wantErr: "Database Host is required",
		},
		{
			name: "missing database",
			mutate: func(in []*core.Connection) []*core.Connection {
				in[1].Value = ""
				return in
			},
			wantErr: "Database is required",
		},
		{
			name: "missing username",
			mutate: func(in []*core.Connection) []*core.Connection {
				in[2].Value = ""
				return in
			},
			wantErr: "Username is required",
		},
		{
			name: "host absent entirely",
			mutate: func(in []*core.Connection) []*core.Connection {
				return in[1:]
			},
			wantErr: "Database Host is required",
		},
		{
			name: "invalid port",
			mutate: func(in []*core.Connection) []*core.Connection {
				return append(in, conn("port", core.ConnectionTypeInteger, 70000))
			},
			wantErr: "is not a valid port number",
		},
		{
			name: "invalid ssl mode",
			mutate: func(in []*core.Connection) []*core.Connection {
				return append(in, conn("ssl_mode", core.ConnectionTypeString, "banana"))
			},
			wantErr: `"banana" is not a valid SSL mode`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)
			_, err := GetAuth(tt.mutate(fullAuthInputs()))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(tt.wantErr))
		})
	}
}

func TestGetAuth_AllSSLModesAccepted(t *testing.T) {
	RegisterTestingT(t)

	for _, opt := range SSLModeOptions {
		inputs := append(fullAuthInputs(), conn("ssl_mode", core.ConnectionTypeString, opt.Value))
		a, err := GetAuth(inputs)
		Expect(err).ToNot(HaveOccurred(), "SSL mode %q is offered in the dropdown but rejected by GetAuth", opt.Value)
		Expect(a.SSLMode).To(Equal(opt.Value))
	}
}

// THE bug OptionalString exists to prevent.
//
// Connection.String() special-cases the text-ish types and otherwise falls
// through to fmt.Sprintf("%v", …) — so a ConnectionTypeSecret that was never
// filled in stringifies to the literal six characters "<nil>", which would then
// be sent to Postgres as the password. Pin both halves: that String() really
// does produce "<nil>", and that OptionalString really does not.
func TestOptionalString_NilSecretIsEmptyNotAngleBracketNil(t *testing.T) {
	RegisterTestingT(t)

	c := &core.Connection{Name: "password", Type: core.ConnectionTypeSecret, Value: nil}

	// The trap, pinned so nobody "simplifies" OptionalString back into String().
	Expect(c.String()).ToNot(BeNil())
	Expect(*c.String()).To(Equal("<nil>"),
		"Connection.String() on an unset secret still renders \"<nil>\" — OptionalString must not use it raw")

	Expect(OptionalString(c)).To(Equal(""))

	// And end to end: an unset password must not reach Auth as "<nil>".
	inputs := []*core.Connection{
		conn("host", core.ConnectionTypeString, "db.example.com"),
		conn("database", core.ConnectionTypeString, "vectordb"),
		conn("username", core.ConnectionTypeString, "postgres"),
		{Name: "password", Type: core.ConnectionTypeSecret, Value: nil},
	}
	a, err := GetAuth(inputs)
	Expect(err).ToNot(HaveOccurred())
	Expect(a.Password).To(Equal(""))
	Expect(a.Password).ToNot(Equal("<nil>"))
	Expect(a.DSN()).ToNot(ContainSubstring("nil"))
}

func TestOptionalString_TrimsAndHandlesNil(t *testing.T) {
	RegisterTestingT(t)

	Expect(OptionalString(nil)).To(Equal(""))
	Expect(OptionalString(conn("x", core.ConnectionTypeString, "  padded  "))).To(Equal("padded"))
	Expect(OptionalString(conn("x", core.ConnectionTypeString, nil))).To(Equal(""))
}

func TestOptionalBoolAndInt(t *testing.T) {
	RegisterTestingT(t)

	Expect(OptionalBool(nil, true)).To(BeTrue())
	Expect(OptionalBool(conn("x", core.ConnectionTypeBoolean, nil), true)).To(BeTrue())
	Expect(OptionalBool(conn("x", core.ConnectionTypeBoolean, false), true)).To(BeFalse())
	// A checkbox bound to a variable arrives as the string "true".
	Expect(OptionalBool(conn("x", core.ConnectionTypeBoolean, "true"), false)).To(BeTrue())

	Expect(OptionalInt(nil, 7)).To(Equal(7))
	Expect(OptionalInt(conn("x", core.ConnectionTypeInteger, nil), 7)).To(Equal(7))
	Expect(OptionalInt(conn("x", core.ConnectionTypeInteger, 12), 7)).To(Equal(12))
	Expect(OptionalInt(conn("x", core.ConnectionTypeInteger, "12"), 7)).To(Equal(12))
}

// ---------------------------------------------------------------------------
// DSN
// ---------------------------------------------------------------------------

// The legacy SQL node builds its DSN with fmt.Sprintf, so a password containing
// '@' or '/' restructures the URL and the connection either fails weirdly or
// lands somewhere it shouldn't. net/url percent-encodes the userinfo, and the
// only way to prove that is to parse the result back.
func TestDSN_PasswordCannotRestructureTheURL(t *testing.T) {
	RegisterTestingT(t)

	const nasty = `p@ss/w:ord?x=1#frag`

	a := Auth{
		Host:     "db.example.com",
		Port:     5432,
		Database: "vectordb",
		Username: "postgres",
		Password: nasty,
		SSLMode:  "disable",
	}
	dsn := a.DSN()

	// The metacharacters must be escaped, not present raw.
	Expect(dsn).ToNot(ContainSubstring(nasty))
	Expect(dsn).To(ContainSubstring("%40")) // @
	Expect(dsn).To(ContainSubstring("%2F")) // /
	Expect(dsn).To(ContainSubstring("%3A")) // :

	// And the URL still means what we meant.
	u, err := url.Parse(dsn)
	Expect(err).ToNot(HaveOccurred())
	Expect(u.Scheme).To(Equal("postgres"))
	Expect(u.Host).To(Equal("db.example.com:5432"))
	Expect(u.Path).To(Equal("/vectordb"))
	Expect(u.User.Username()).To(Equal("postgres"))
	pw, set := u.User.Password()
	Expect(set).To(BeTrue())
	Expect(pw).To(Equal(nasty), "the password must survive the escape/unescape round-trip byte for byte")
	Expect(u.Fragment).To(Equal(""), "the '#' in the password must not become a URL fragment")

	q := u.Query()
	Expect(q.Get("sslmode")).To(Equal("disable"))
	Expect(q.Get("connect_timeout")).To(Equal("5"),
		"without connect_timeout a black-holed host hangs until the OS TCP timeout")
	Expect(q.Get("x")).To(Equal(""), "the password must not smuggle in extra libpq parameters")
}

func TestDSN_Basics(t *testing.T) {
	RegisterTestingT(t)

	a := Auth{Host: "10.0.0.5", Port: 6432, Database: "rag", Username: "u", Password: "p", SSLMode: "require"}
	u, err := url.Parse(a.DSN())
	Expect(err).ToNot(HaveOccurred())
	Expect(u.Host).To(Equal("10.0.0.5:6432"))
	Expect(u.Query().Get("sslmode")).To(Equal("require"))
	Expect(u.Query().Get("connect_timeout")).To(Equal("5"))
}

// ---------------------------------------------------------------------------
// Metric / Similarity
// ---------------------------------------------------------------------------

// Get the ops class wrong and the index is silently never used — the query
// still returns the right answer, just via a sequential scan over every row.
// So the operator and the ops class have to agree, per metric.
func TestMetric(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		in       string
		operator string
		opsClass string
	}{
		{"", "<=>", "vector_cosine_ops"}, // default
		{"cosine", "<=>", "vector_cosine_ops"},
		{"inner_product", "<#>", "vector_ip_ops"},
		{"euclidean", "<->", "vector_l2_ops"},
		{"  cosine  ", "<=>", "vector_cosine_ops"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			RegisterTestingT(t)
			m, err := Metric(tt.in)
			Expect(err).ToNot(HaveOccurred())
			Expect(m.Operator).To(Equal(tt.operator))
			Expect(m.OpsClass).To(Equal(tt.opsClass))
		})
	}

	_, err := Metric("manhattan")
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("cosine, inner_product or euclidean"))
}

// Every metric offered in the dropdown must resolve.
func TestMetric_AllDropdownOptionsResolve(t *testing.T) {
	RegisterTestingT(t)

	for _, opt := range DistanceMetricOptions {
		m, err := Metric(opt.Value)
		Expect(err).ToNot(HaveOccurred(), "metric %q is offered in the dropdown but not resolvable", opt.Value)
		Expect(m.Operator).ToNot(BeEmpty())
		Expect(m.OpsClass).ToNot(BeEmpty())
	}
}

// min_score always filters on the similarity score, never the raw distance, so
// the score MUST be monotonically decreasing in distance for every metric — a
// metric where it isn't would make the threshold mean the opposite of what the
// operator intended.
func TestSimilarity_MonotonicallyDecreasingInDistance(t *testing.T) {
	RegisterTestingT(t)

	for _, name := range []string{"cosine", "euclidean", "inner_product"} {
		t.Run(name, func(t *testing.T) {
			RegisterTestingT(t)
			prev := Similarity(name, 0)
			for _, d := range []float64{0.001, 0.01, 0.1, 0.5, 1, 1.5, 2, 10, 100} {
				got := Similarity(name, d)
				Expect(got).To(BeNumerically("<", prev),
					"%s: similarity must fall as distance rises (d=%v)", name, d)
				prev = got
			}
		})
	}
}

func TestSimilarity_Values(t *testing.T) {
	RegisterTestingT(t)

	// Cosine: an identical vector has distance 0, which must score a clean 1.
	Expect(Similarity("cosine", 0)).To(Equal(1.0))
	Expect(Similarity("cosine", 1)).To(Equal(0.5)) // orthogonal
	Expect(Similarity("cosine", 2)).To(Equal(0.0)) // opposite
	Expect(Similarity("", 0)).To(Equal(1.0))       // unknown name falls back to cosine
	Expect(Similarity("euclidean", 0)).To(Equal(1.0))
	Expect(Similarity("euclidean", 1)).To(Equal(0.5))
	// pgvector's <#> returns the NEGATIVE inner product, so a strong match is a
	// large negative distance and the score is its negation.
	Expect(Similarity("inner_product", -0.9)).To(Equal(0.9))
	Expect(Similarity("inner_product", 0.3)).To(Equal(-0.3))
}

// ---------------------------------------------------------------------------
// CheckDimension
// ---------------------------------------------------------------------------

func TestCheckDimension(t *testing.T) {
	RegisterTestingT(t)

	vec1536 := make([]float32, 1536)

	// Matching.
	Expect(CheckDimension(1536, vec1536, "public.docs")).To(Succeed())

	// An unbounded `vector` column (what n8n creates) declares no dimension, so
	// there is no preflight to do — and this must NOT be reported as an error.
	Expect(CheckDimension(-1, vec1536, "public.docs")).To(Succeed())
	Expect(CheckDimension(0, vec1536, "public.docs")).To(Succeed())

	// No vector to check.
	Expect(CheckDimension(1536, nil, "public.docs")).To(Succeed())
	Expect(CheckDimension(1536, []float32{}, "public.docs")).To(Succeed())
}

func TestCheckDimension_MismatchNamesBothNumbers(t *testing.T) {
	RegisterTestingT(t)

	err := CheckDimension(1024, make([]float32, 1536), "public.docs")
	Expect(err).To(HaveOccurred())

	msg := err.Error()
	Expect(msg).To(ContainSubstring("1536"), "the message must name the embedding's dimension")
	Expect(msg).To(ContainSubstring("1024"), "the message must name the table's dimension")
	Expect(msg).To(ContainSubstring("public.docs"))
	// It has to explain the cause, not just the numbers.
	Expect(msg).To(ContainSubstring("embedding model"))
}

// ---------------------------------------------------------------------------
// Humanise / Redact
// ---------------------------------------------------------------------------

func testAuth() Auth {
	return Auth{
		Host: "db.example.com", Port: 5432, Database: "vectordb",
		Username: "postgres", Password: "hunter2", SSLMode: "disable",
	}
}

func TestHumanise_DimensionMismatchExplainsTheModel(t *testing.T) {
	RegisterTestingT(t)

	err := &pq.Error{Code: "22000", Message: "different vector dimensions 1024 and 1536"}
	msg := Humanise(testAuth(), err)

	Expect(msg).To(ContainSubstring("different vector dimensions 1024 and 1536"))
	Expect(msg).To(ContainSubstring("embedding model"),
		"a raw \"different vector dimensions 1024 and 1536\" means nothing to a front-of-house operator")
	Expect(msg).To(ContainSubstring("1536")) // text-embedding-3-small
	Expect(msg).To(ContainSubstring("1024")) // Titan v2
}

func TestHumanise_UndefinedTableMentionsCaseSensitivity(t *testing.T) {
	RegisterTestingT(t)

	err := &pq.Error{Code: "42P01", Message: `relation "MyTable" does not exist`}
	msg := Humanise(testAuth(), err)

	Expect(msg).To(ContainSubstring("case-sensitive"),
		"quoting makes identifiers case-sensitive, which is the usual cause of 42P01 here")
	Expect(msg).To(ContainSubstring("dropdown"))
}

func TestHumanise_BadPasswordMentionsTheUsername(t *testing.T) {
	RegisterTestingT(t)

	for _, code := range []string{"28P01", "28000"} {
		msg := Humanise(testAuth(), &pq.Error{Code: pq.ErrorCode(code), Message: "password authentication failed"})
		Expect(msg).To(ContainSubstring(`"postgres"`), "%s must name the user it tried to log in as", code)
		Expect(msg).To(ContainSubstring("username and password"))
		Expect(msg).ToNot(ContainSubstring("hunter2"))
	}
}

func TestHumanise_OtherCodes(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name string
		err  *pq.Error
		want string
	}{
		{"extension missing", &pq.Error{Code: "42704", Message: `type "vector" does not exist`}, "CREATE EXTENSION vector"},
		{"insufficient privilege", &pq.Error{Code: "42501", Message: "permission denied for table docs"}, `"postgres"`},
		{"unknown database", &pq.Error{Code: "3D000", Message: "database does not exist"}, `"vectordb"`},
		{"query cancelled", &pq.Error{Code: "57014", Message: "canceling statement"}, "Create Index"},
		{"unique violation", &pq.Error{Code: "23505", Message: "duplicate key value"}, "Upsert Document"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)
			Expect(Humanise(testAuth(), tt.err)).To(ContainSubstring(tt.want))
		})
	}
}

func TestHumanise_NilAndPassthrough(t *testing.T) {
	RegisterTestingT(t)

	Expect(Humanise(testAuth(), nil)).To(Equal(""))

	// An error we did not anticipate falls through unchanged — inventing a
	// friendly message for it would only hide it.
	Expect(Humanise(testAuth(), context.DeadlineExceeded)).To(ContainSubstring("didn't respond within"))
}

func TestRedact(t *testing.T) {
	RegisterTestingT(t)

	a := testAuth()

	// Everywhere it appears, not just the first.
	msg := "dial postgres://postgres:hunter2@db:5432: bad password hunter2 (hunter2)"
	got := Redact(a, msg)
	Expect(got).ToNot(ContainSubstring("hunter2"))
	Expect(strings.Count(got, "********")).To(Equal(3))

	// A blank password must not turn every empty string into asterisks.
	a.Password = ""
	Expect(Redact(a, "no secrets here")).To(Equal("no secrets here"))
}

// ---------------------------------------------------------------------------
// ResolveColumns (go-sqlmock)
// ---------------------------------------------------------------------------

const describeSQL = "pg_attribute"
const pkSQL = "indisprimary"

func columnRows(cols ...[3]string) *sqlmock.Rows {
	r := sqlmock.NewRows([]string{"attname", "typname", "formatted"})
	for _, c := range cols {
		r.AddRow(c[0], c[1], c[2])
	}
	return r
}

// n8n / LangChain's shape: id / text / metadata / embedding, with an unbounded
// `vector` column.
func n8nTable() *sqlmock.Rows {
	return columnRows(
		[3]string{"id", "uuid", "uuid"},
		[3]string{"text", "text", "text"},
		[3]string{"metadata", "jsonb", "jsonb"},
		[3]string{"embedding", "vector", "vector"},
	)
}

// A hand-rolled table: id / content / metadata / embedding, dimensioned.
func handRolledTable() *sqlmock.Rows {
	return columnRows(
		[3]string{"id", "int8", "bigint"},
		[3]string{"content", "text", "text"},
		[3]string{"metadata", "jsonb", "jsonb"},
		[3]string{"embedding", "vector", "vector(1536)"},
	)
}

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	Expect(err).ToNot(HaveOccurred())
	t.Cleanup(func() { db.Close() })
	return db, mock
}

// (a) Full auto-detect on an n8n-shaped table — the "point it at the table
// someone else built and it just works" promise.
func TestResolveColumns_AutoDetectN8nShape(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "documents").WillReturnRows(n8nTable())
	mock.ExpectQuery(pkSQL).WithArgs("public", "documents").WillReturnRows(
		sqlmock.NewRows([]string{"attname"}).AddRow("id"))

	cols, err := ResolveColumns(context.Background(), db, "public", "documents", ColumnInputs{})
	Expect(err).ToNot(HaveOccurred())

	Expect(cols.ID).To(Equal("id"))
	Expect(cols.Content).To(Equal("text"))
	Expect(cols.Metadata).To(Equal("metadata"))
	Expect(cols.Vector).To(Equal("embedding"))
	Expect(cols.HasMetadata()).To(BeTrue())

	// Everything is quoted exactly once, here, so no caller has to remember to.
	Expect(cols.QID).To(Equal(`"id"`))
	Expect(cols.QContent).To(Equal(`"text"`))
	Expect(cols.QMetadata).To(Equal(`"metadata"`))
	Expect(cols.QVector).To(Equal(`"embedding"`))

	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// (b) The user's real shape: content, not text.
func TestResolveColumns_AutoDetectContentColumn(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("rag", "docs").WillReturnRows(handRolledTable())
	mock.ExpectQuery(pkSQL).WithArgs("rag", "docs").WillReturnRows(
		sqlmock.NewRows([]string{"attname"}).AddRow("id"))

	cols, err := ResolveColumns(context.Background(), db, "rag", "docs", ColumnInputs{})
	Expect(err).ToNot(HaveOccurred())
	Expect(cols.Content).To(Equal("content"))
	Expect(cols.Vector).To(Equal("embedding"))
	Expect(cols.Metadata).To(Equal("metadata"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// (c) No vector column at all — this isn't a pgvector table yet, and the error
// has to say what to do about it.
func TestResolveColumns_NoVectorColumn(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "users").WillReturnRows(columnRows(
		[3]string{"id", "int8", "bigint"},
		[3]string{"name", "text", "text"},
	))

	_, err := ResolveColumns(context.Background(), db, "public", "users", ColumnInputs{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("isn't a pgvector table yet"))
	Expect(err.Error()).To(ContainSubstring("Create Table"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// (d) Two vector columns — we cannot guess, so say so and name both.
func TestResolveColumns_TwoVectorColumns(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "docs").WillReturnRows(columnRows(
		[3]string{"id", "int8", "bigint"},
		[3]string{"content", "text", "text"},
		[3]string{"embedding", "vector", "vector(1536)"},
		[3]string{"title_embedding", "vector", "vector(1536)"},
	))

	_, err := ResolveColumns(context.Background(), db, "public", "docs", ColumnInputs{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("more than one vector column"))
	Expect(err.Error()).To(ContainSubstring("embedding"))
	Expect(err.Error()).To(ContainSubstring("title_embedding"))
	Expect(err.Error()).To(ContainSubstring("Embedding Column"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// Two vector columns stop being ambiguous the moment the operator names one.
func TestResolveColumns_TwoVectorColumnsDisambiguated(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "docs").WillReturnRows(columnRows(
		[3]string{"id", "int8", "bigint"},
		[3]string{"content", "text", "text"},
		[3]string{"embedding", "vector", "vector(1536)"},
		[3]string{"title_embedding", "vector", "vector(1536)"},
	))
	mock.ExpectQuery(pkSQL).WithArgs("public", "docs").WillReturnRows(
		sqlmock.NewRows([]string{"attname"}).AddRow("id"))

	cols, err := ResolveColumns(context.Background(), db, "public", "docs",
		ColumnInputs{Vector: "title_embedding"})
	Expect(err).ToNot(HaveOccurred())
	Expect(cols.Vector).To(Equal("title_embedding"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// (e) An explicit column that doesn't exist — list what IS there, because the
// operator has almost certainly just typo'd or picked the wrong table.
func TestResolveColumns_ExplicitColumnMissing(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "docs").WillReturnRows(handRolledTable())

	_, err := ResolveColumns(context.Background(), db, "public", "docs",
		ColumnInputs{Vector: "embeddings"}) // note the plural
	Expect(err).To(HaveOccurred())

	msg := err.Error()
	Expect(msg).To(ContainSubstring(`"embeddings" doesn't exist`))
	Expect(msg).To(ContainSubstring("available columns are"))
	Expect(msg).To(ContainSubstring("id, content, metadata, embedding"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// (f) A table with no metadata column searches perfectly well — it just cannot
// filter. That is a capability gap, NOT an error.
func TestResolveColumns_NoMetadataColumnIsNotAnError(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "docs").WillReturnRows(columnRows(
		[3]string{"id", "int8", "bigint"},
		[3]string{"content", "text", "text"},
		[3]string{"embedding", "vector", "vector(1536)"},
	))
	mock.ExpectQuery(pkSQL).WithArgs("public", "docs").WillReturnRows(
		sqlmock.NewRows([]string{"attname"}).AddRow("id"))

	cols, err := ResolveColumns(context.Background(), db, "public", "docs", ColumnInputs{})
	Expect(err).ToNot(HaveOccurred())
	Expect(cols.HasMetadata()).To(BeFalse())
	Expect(cols.Metadata).To(Equal(""))
	Expect(cols.QMetadata).To(Equal(""))
	Expect(cols.Vector).To(Equal("embedding"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

func TestResolveColumns_TableDoesNotExist(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "nope").WillReturnRows(columnRows())

	_, err := ResolveColumns(context.Background(), db, "public", "nope", ColumnInputs{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("doesn't exist, or this user can't see it"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// An explicit embedding column that is not actually a vector is a different
// mistake from one that doesn't exist, and gets its own message.
func TestResolveColumns_ExplicitVectorColumnIsNotAVector(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "docs").WillReturnRows(handRolledTable())

	_, err := ResolveColumns(context.Background(), db, "public", "docs",
		ColumnInputs{Vector: "content"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("is a text, not a vector"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// Metadata has to live in jsonb — a text column holding JSON would not support
// any of the -> / ->> / @> filtering.
func TestResolveColumns_ExplicitMetadataColumnIsNotJSONB(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "docs").WillReturnRows(handRolledTable())

	_, err := ResolveColumns(context.Background(), db, "public", "docs",
		ColumnInputs{Metadata: "content"})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("not jsonb"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// No primary key and no "id" column: we cannot invent one, so ask.
func TestResolveColumns_NoIDColumn(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "docs").WillReturnRows(columnRows(
		[3]string{"uid", "uuid", "uuid"},
		[3]string{"content", "text", "text"},
		[3]string{"embedding", "vector", "vector(1536)"},
	))
	mock.ExpectQuery(pkSQL).WithArgs("public", "docs").WillReturnRows(sqlmock.NewRows([]string{"attname"}))

	_, err := ResolveColumns(context.Background(), db, "public", "docs", ColumnInputs{})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("no primary key"))
	Expect(err.Error()).To(ContainSubstring("ID Column"))
	Expect(err.Error()).To(ContainSubstring("uid, content, embedding"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// The primary key wins over a column merely named "id".
func TestResolveColumns_PrimaryKeyWinsOverNamedID(t *testing.T) {
	RegisterTestingT(t)

	db, mock := newMockDB(t)
	mock.ExpectQuery(describeSQL).WithArgs("public", "docs").WillReturnRows(columnRows(
		[3]string{"doc_key", "text", "text"},
		[3]string{"id", "int8", "bigint"},
		[3]string{"content", "text", "text"},
		[3]string{"embedding", "vector", "vector(1536)"},
	))
	mock.ExpectQuery(pkSQL).WithArgs("public", "docs").WillReturnRows(
		sqlmock.NewRows([]string{"attname"}).AddRow("doc_key"))

	cols, err := ResolveColumns(context.Background(), db, "public", "docs", ColumnInputs{})
	Expect(err).ToNot(HaveOccurred())
	Expect(cols.ID).To(Equal("doc_key"))
	Expect(mock.ExpectationsWereMet()).To(Succeed())
}

// ---------------------------------------------------------------------------
// TableDimension
// ---------------------------------------------------------------------------

func TestTableDimension(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name   string
		column string
		want   int
	}{
		{"dimensioned vector(1536)", "embedding", 1536},
		{"bare vector (what n8n creates)", "loose", -1},
		{"absent column", "nope", 0},
		{"not a vector", "content", 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)

			db, mock := newMockDB(t)
			mock.ExpectQuery(describeSQL).WithArgs("public", "docs").WillReturnRows(columnRows(
				[3]string{"id", "int8", "bigint"},
				[3]string{"content", "text", "text"},
				[3]string{"embedding", "vector", "vector(1536)"},
				[3]string{"loose", "vector", "vector"},
			))

			got, err := TableDimension(context.Background(), db, "public", "docs", tt.column)
			Expect(err).ToNot(HaveOccurred())
			Expect(got).To(Equal(tt.want))
			Expect(mock.ExpectationsWereMet()).To(Succeed())
		})
	}
}

// ---------------------------------------------------------------------------
// Result shapers
// ---------------------------------------------------------------------------

func TestOKAndErrorResult(t *testing.T) {
	RegisterTestingT(t)

	ok := OK(map[string]interface{}{"count": 3}, "Found 3 documents")
	Expect(ok["success"]).To(BeTrue())
	Expect(ok["error"]).To(Equal(""))
	Expect(ok["tool_result"]).To(Equal("Found 3 documents"))
	Expect(ok["count"]).To(Equal(3))

	// OK must cope with a nil map rather than panicking.
	Expect(OK(nil, "done")["success"]).To(BeTrue())

	bad := ErrorResult("it broke")
	Expect(bad["success"]).To(BeFalse())
	Expect(bad["error"]).To(Equal("it broke"))
	Expect(bad["tool_result"]).To(Equal("it broke"))
}

// Fail must never leak the password, and must return a nil error so the engine
// records the failure as data on the error port.
func TestFail_RedactsAndSoftFails(t *testing.T) {
	RegisterTestingT(t)

	out, err := Fail(testAuth(), &pq.Error{Code: "28P01", Message: "password authentication failed"})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(BeFalse())
	Expect(out["error"]).To(ContainSubstring("postgres"))
	Expect(out["error"]).ToNot(ContainSubstring("hunter2"))

	out, err = Failf("no such table %q", "docs")
	Expect(err).ToNot(HaveOccurred())
	Expect(out["error"]).To(Equal(`no such table "docs"`))
}

func TestClamp(t *testing.T) {
	RegisterTestingT(t)

	Expect(Clamp(0, 10, 100)).To(Equal(10))    // unset -> default
	Expect(Clamp(-5, 10, 100)).To(Equal(10))   // negative -> default
	Expect(Clamp(50, 10, 100)).To(Equal(50))   // in range
	Expect(Clamp(500, 10, 100)).To(Equal(100)) // over max -> max
	Expect(Clamp(100, 10, 100)).To(Equal(100)) // exactly max
}

func TestPreview(t *testing.T) {
	RegisterTestingT(t)

	Expect(Preview("  hello   world \n more ")).To(Equal("hello world more"))

	long := strings.Repeat("a", 500)
	got := Preview(long)
	Expect(got).To(HaveLen(summaryPreview + len("…")))
	Expect(strings.HasSuffix(got, "…")).To(BeTrue())
}

// !!! FAILING — REAL BUG IN THE FOUNDATION (actions/vectordatabase/pgvector/common.go, Preview) !!!
//
// Preview truncates with `s[:summaryPreview]`, which is a BYTE offset. When byte
// 200 lands in the middle of a multi-byte rune, the preview is cut mid-rune and
// is no longer valid UTF-8.
//
// This is not theoretical: it fires on any document that isn't pure ASCII. "€"
// is 3 bytes, so a document of euro signs breaks on the 67th; CJK, emoji, and
// (for a UK/EU customer base) plain accented Latin all hit it as soon as an odd
// number of continuation bytes straddles the cut.
//
// It matters because Preview's output goes into `tool_result` — which is the
// human-facing summary shown in the UI, AND the text handed to an LLM when a
// pgvector step runs as an agent tool. encoding/json silently rewrites the
// invalid bytes to U+FFFD, so the operator sees "…€€��…" and nothing ever errors.
//
// Fix (one line, in common.go — deliberately NOT applied here):
//
//	cut := summaryPreview
//	for cut > 0 && !utf8.RuneStart(s[cut]) {
//	    cut--
//	}
//	return s[:cut] + "…"
//
// or simply truncate on runes rather than bytes.
// Whether a given document happens to break depends on where byte 200 lands, so
// each case is swept across four byte alignments with a leading ASCII run. Any
// rune wider than one byte is guaranteed to straddle the cut at some alignment —
// there is no multi-byte script that is safe here, only lucky inputs.
func TestPreview_TruncatesOnARuneBoundary(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name string
		body string
	}{
		{"accented latin (2 bytes)", strings.Repeat("é", 200)},
		{"euro signs (3 bytes)", strings.Repeat("€", 200)},
		{"CJK (3 bytes)", strings.Repeat("文", 200)},
		{"emoji (4 bytes)", strings.Repeat("🙂", 200)},
		{"realistic prose", strings.Repeat("naïve café €5 文書 🙂 ", 40)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)

			for align := 0; align < 4; align++ {
				doc := strings.Repeat("x", align) + tt.body
				Expect(utf8.ValidString(doc)).To(BeTrue(), "the test input itself must be valid UTF-8")

				got := Preview(doc)
				Expect(utf8.ValidString(got)).To(BeTrue(),
					"alignment %d: Preview cut a multi-byte rune in half — the result is not valid UTF-8: %q. "+
						"tool_result is shown to the operator and fed to an LLM when the step runs as an agent "+
						"tool; truncate on a rune boundary, not a byte one.",
					align, got)
			}
		})
	}
}
