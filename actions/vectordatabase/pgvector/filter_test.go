package pgvector

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	"github.com/lib/pq"
	. "github.com/onsi/gomega"
)

// metaCols is a table that HAS a metadata column, which is the precondition for
// any filtering at all.
func metaCols() ColumnSet {
	return ColumnSet{
		ID: "id", Content: "content", Metadata: "metadata", Vector: "embedding",
		QID: `"id"`, QContent: `"content"`, QMetadata: `"metadata"`, QVector: `"embedding"`,
	}
}

// noMetaCols is the same table without one.
func noMetaCols() ColumnSet {
	return ColumnSet{
		ID: "id", Content: "content", Vector: "embedding",
		QID: `"id"`, QContent: `"content"`, QVector: `"embedding"`,
	}
}

// kvFilter builds the metadata_filter connection the way an action passes it in.
func kvFilter(v any) []*core.Connection {
	return []*core.Connection{{Name: "metadata_filter", Type: core.ConnectionTypeKeyValueArray, Value: v}}
}

// ---------------------------------------------------------------------------
// THE headline test
// ---------------------------------------------------------------------------

// LangChain — and therefore shipped n8n — builds this clause as
//
//	`${this.metadataColumnName}->>'${key}' = $${paramCount}`
//
// with the key coming from an expression-capable UI field. That is a live SQL
// injection: a filter key of `x'; DROP TABLE t; --` lands verbatim in the WHERE
// clause. `metadata->>$1 = $2` is the same query with none of the hole.
//
// This test is the proof we closed it: not one byte of the injected key may
// appear in the generated SQL — it must appear in Args instead, where Postgres
// treats it as data and can never parse it as syntax.
func TestBuildFilter_KeyIsBoundNeverInterpolated(t *testing.T) {
	RegisterTestingT(t)

	const injected = `x'; DROP TABLE t; --`

	advanced := fmt.Sprintf(`{%q: {"eq": "1"}}`, injected)
	f, err := BuildFilter(metaCols(), nil, advanced, 0)
	Expect(err).ToNot(HaveOccurred())

	// The key is nowhere in the SQL — not whole, and not in fragments.
	Expect(f.SQL).ToNot(ContainSubstring(injected))
	for _, frag := range []string{"DROP", "TABLE", "--", "'", ";", "x'"} {
		Expect(f.SQL).ToNot(ContainSubstring(frag),
			"the filter KEY leaked into the SQL text: %q contains %q", f.SQL, frag)
	}

	// It is bound instead.
	Expect(f.SQL).To(Equal(`"metadata"->>$1 = $2`))
	Expect(f.Args).To(Equal([]interface{}{injected, "1"}))

	// The same holds for the simple key/value grid.
	f, err = BuildFilter(metaCols(),
		kvFilter([]map[string]any{{"key": injected, "value": "1"}}), "", 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).ToNot(ContainSubstring("DROP"))
	Expect(f.SQL).To(Equal(`"metadata"->>$1 = $2`))
	Expect(f.Args[0]).To(Equal(injected))
}

// The same for a filter VALUE, and for every operator that takes one — a bound
// value can never be parsed as syntax whatever it holds.
func TestBuildFilter_ValueIsBoundNeverInterpolated(t *testing.T) {
	RegisterTestingT(t)

	const injected = `handbook'); DROP TABLE documents; --`

	for _, op := range []string{"eq", "neq", "like", "ilike"} {
		t.Run(op, func(t *testing.T) {
			RegisterTestingT(t)
			advanced := fmt.Sprintf(`{"source": {%q: %q}}`, op, injected)
			f, err := BuildFilter(metaCols(), nil, advanced, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.SQL).ToNot(ContainSubstring("DROP"))
			Expect(f.SQL).ToNot(ContainSubstring("'"))
			Expect(f.Args).To(ContainElement(injected))
		})
	}
}

// ---------------------------------------------------------------------------
// Operators
// ---------------------------------------------------------------------------

func TestBuildFilter_Operators(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name     string
		advanced string
		wantSQL  string
		wantArgs []interface{}
	}{
		{
			name:     "eq",
			advanced: `{"source": {"eq": "handbook"}}`,
			wantSQL:  `"metadata"->>$1 = $2`,
			wantArgs: []interface{}{"source", "handbook"},
		},
		{
			// Shorthand: a bare scalar means eq.
			name:     "bare scalar is eq",
			advanced: `{"source": "handbook"}`,
			wantSQL:  `"metadata"->>$1 = $2`,
			wantArgs: []interface{}{"source", "handbook"},
		},
		{
			// A number compared through ->> is text on both sides.
			name:     "eq against a number binds its text form",
			advanced: `{"page": {"eq": 3}}`,
			wantSQL:  `"metadata"->>$1 = $2`,
			wantArgs: []interface{}{"page", "3"},
		},
		{
			// NULL-tolerant, matching LangChain: a document with no "tag" key at
			// all DOES satisfy `tag != draft`. A plain `<> $2` would drop it,
			// because NULL <> 'draft' is NULL, not true.
			name:     "neq is NULL-tolerant",
			advanced: `{"tag": {"neq": "draft"}}`,
			wantSQL:  `("metadata"->>$1 IS NULL OR "metadata"->>$3 <> $2)`,
			wantArgs: []interface{}{"tag", "draft", "tag"},
		},
		{
			name:     "gt casts to numeric",
			advanced: `{"page": {"gt": 3}}`,
			wantSQL:  `("metadata"->>$1)::numeric > $2`,
			wantArgs: []interface{}{"page", float64(3)},
		},
		{
			name:     "gte",
			advanced: `{"page": {"gte": 3}}`,
			wantSQL:  `("metadata"->>$1)::numeric >= $2`,
			wantArgs: []interface{}{"page", float64(3)},
		},
		{
			name:     "lt",
			advanced: `{"price": {"lt": 9.99}}`,
			wantSQL:  `("metadata"->>$1)::numeric < $2`,
			wantArgs: []interface{}{"price", float64(9.99)},
		},
		{
			name:     "lte",
			advanced: `{"price": {"lte": 9.99}}`,
			wantSQL:  `("metadata"->>$1)::numeric <= $2`,
			wantArgs: []interface{}{"price", float64(9.99)},
		},
		{
			name:     "like",
			advanced: `{"title": {"like": "%policy%"}}`,
			wantSQL:  `"metadata"->>$1 LIKE $2`,
			wantArgs: []interface{}{"title", "%policy%"},
		},
		{
			name:     "ilike",
			advanced: `{"title": {"ilike": "%policy%"}}`,
			wantSQL:  `"metadata"->>$1 ILIKE $2`,
			wantArgs: []interface{}{"title", "%policy%"},
		},
		{
			// jsonb's own ? operator — not a placeholder, lib/pq only knows $n.
			name:     "exists true",
			advanced: `{"tag": {"exists": true}}`,
			wantSQL:  `"metadata" ? $1`,
			wantArgs: []interface{}{"tag"},
		},
		{
			name:     "exists false",
			advanced: `{"tag": {"exists": false}}`,
			wantSQL:  `NOT ("metadata" ? $1)`,
			wantArgs: []interface{}{"tag"},
		},
		{
			name:     "isNull true",
			advanced: `{"tag": {"isNull": true}}`,
			wantSQL:  `"metadata"->>$1 IS NULL`,
			wantArgs: []interface{}{"tag"},
		},
		{
			name:     "isNull false",
			advanced: `{"tag": {"isNull": false}}`,
			wantSQL:  `"metadata"->>$1 IS NOT NULL`,
			wantArgs: []interface{}{"tag"},
		},
		{
			// Whole-object containment against the metadata column — the field
			// name is not part of the predicate, the operand is the whole
			// sub-document to match.
			name:     "contains",
			advanced: `{"meta": {"contains": {"a":1}}}`,
			wantSQL:  `"metadata" @> $1::jsonb`,
			wantArgs: []interface{}{`{"a":1}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)
			f, err := BuildFilter(metaCols(), nil, tt.advanced, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.SQL).To(Equal(tt.wantSQL))
			Expect(f.Args).To(Equal(tt.wantArgs))
		})
	}
}

// The ::numeric cast is the whole point of gt/gte/lt/lte: without it Postgres
// compares the ->> output as TEXT, where "10" < "9". A filter of page > 9 would
// then silently drop page 10.
func TestBuildFilter_ComparisonsCastToNumeric(t *testing.T) {
	RegisterTestingT(t)

	for _, op := range []string{"gt", "gte", "lt", "lte"} {
		f, err := BuildFilter(metaCols(), nil, fmt.Sprintf(`{"page": {%q: 9}}`, op), 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(f.SQL).To(ContainSubstring("::numeric"),
			"%s must cast, or Postgres compares \"10\" < \"9\" as text", op)
		// The bound value stays a number, not the text jsonb's ->> would yield.
		Expect(f.Args[1]).To(Equal(float64(9)))
	}

	// A non-numeric operand is a mistake worth naming.
	_, err := BuildFilter(metaCols(), nil, `{"page": {"gt": "three"}}`, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("page.gt must be a number"))
}

// in / notIn bind the whole list as a single pq.Array parameter and cast it to
// text[] — one placeholder, however many values, and none of them interpolated.
func TestBuildFilter_InAndNotIn(t *testing.T) {
	RegisterTestingT(t)

	f, err := BuildFilter(metaCols(), nil, `{"tag": {"in": ["a","b","c"]}}`, 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).To(Equal(`"metadata"->>$1 = ANY($2::text[])`))
	Expect(f.Args).To(HaveLen(2))
	Expect(f.Args[0]).To(Equal("tag"))
	Expect(f.Args[1]).To(Equal(pq.Array([]string{"a", "b", "c"})))

	f, err = BuildFilter(metaCols(), nil, `{"tag": {"notIn": ["a","b"]}}`, 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).To(Equal(`NOT ("metadata"->>$1 = ANY($2::text[]))`))
	Expect(f.Args[1]).To(Equal(pq.Array([]string{"a", "b"})))

	// Mixed JSON scalars all render as the text ->> would produce.
	f, err = BuildFilter(metaCols(), nil, `{"n": {"in": [1, true, "x"]}}`, 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.Args[1]).To(Equal(pq.Array([]string{"1", "true", "x"})))

	_, err = BuildFilter(metaCols(), nil, `{"tag": {"in": "a"}}`, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("tag.in must be a list of values"))
}

func TestBuildFilter_ArrayContains(t *testing.T) {
	RegisterTestingT(t)

	f, err := BuildFilter(metaCols(), nil, `{"tags": {"arrayContains": ["red","blue"]}}`, 0)
	Expect(err).ToNot(HaveOccurred())
	// -> (not ->>) so the left side stays jsonb for the ?| operator.
	Expect(f.SQL).To(Equal(`"metadata"->$1 ?| $2::text[]`))
	Expect(f.Args[0]).To(Equal("tags"))
	Expect(f.Args[1]).To(Equal(pq.Array([]string{"red", "blue"})))

	_, err = BuildFilter(metaCols(), nil, `{"tags": {"arrayContains": "red"}}`, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("tags.arrayContains must be a list of values"))
}

// Several operators on one field AND together, inside their own parentheses.
func TestBuildFilter_MultipleOpsOnOneField(t *testing.T) {
	RegisterTestingT(t)

	f, err := BuildFilter(metaCols(), nil, `{"page": {"gte": 3, "lte": 10}}`, 0)
	Expect(err).ToNot(HaveOccurred())
	// Ops are sorted for deterministic SQL: gte before lte.
	Expect(f.SQL).To(Equal(`(("metadata"->>$1)::numeric >= $2 AND ("metadata"->>$3)::numeric <= $4)`))
	Expect(f.Args).To(Equal([]interface{}{"page", float64(3), "page", float64(10)}))
}

// ---------------------------------------------------------------------------
// Grouping
// ---------------------------------------------------------------------------

func TestBuildFilter_AndOrGrouping(t *testing.T) {
	RegisterTestingT(t)

	f, err := BuildFilter(metaCols(), nil,
		`{"$or": [{"status": {"eq": "live"}}, {"pinned": {"eq": true}}]}`, 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).To(Equal(`("metadata"->>$1 = $2 OR "metadata"->>$3 = $4)`))
	Expect(f.Args).To(Equal([]interface{}{"status", "live", "pinned", "true"}))

	f, err = BuildFilter(metaCols(), nil,
		`{"$and": [{"a": {"eq": "1"}}, {"b": {"eq": "2"}}]}`, 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).To(Equal(`("metadata"->>$1 = $2 AND "metadata"->>$3 = $4)`))
}

// A top-level field alongside a group ANDs with it — keys are sorted, and "$or"
// sorts before a bare field name.
func TestBuildFilter_GroupAlongsideField(t *testing.T) {
	RegisterTestingT(t)

	f, err := BuildFilter(metaCols(), nil,
		`{"source": {"eq": "handbook"}, "$or": [{"a": {"eq": "1"}}, {"b": {"eq": "2"}}]}`, 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).To(Equal(
		`("metadata"->>$1 = $2 OR "metadata"->>$3 = $4) AND "metadata"->>$5 = $6`))
	Expect(f.Args).To(Equal([]interface{}{"a", "1", "b", "2", "source", "handbook"}))
}

func TestBuildFilter_NestedGroups(t *testing.T) {
	RegisterTestingT(t)

	f, err := BuildFilter(metaCols(), nil, `{
		"$and": [
			{"status": {"eq": "live"}},
			{"$or": [{"tag": {"eq": "a"}}, {"tag": {"eq": "b"}}]}
		]
	}`, 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).To(Equal(
		`("metadata"->>$1 = $2 AND ("metadata"->>$3 = $4 OR "metadata"->>$5 = $6))`))
	Expect(f.Args).To(Equal([]interface{}{"status", "live", "tag", "a", "tag", "b"}))
}

func TestBuildFilter_GroupMustBeAList(t *testing.T) {
	RegisterTestingT(t)

	_, err := BuildFilter(metaCols(), nil, `{"$or": {"a": {"eq": "1"}}}`, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("$or must be a list of filter objects"))
}

// ---------------------------------------------------------------------------
// startArg — the subtlest bug in the file
// ---------------------------------------------------------------------------

// The caller has already bound its own parameters (the query vector is $1), so
// the filter's placeholders must CONTINUE that sequence rather than restart at
// $1. Get this wrong and Postgres binds the query vector where the filter key
// should be — the query runs, and returns the wrong rows.
func TestBuildFilter_StartArgContinuesTheCallersSequence(t *testing.T) {
	RegisterTestingT(t)

	// document_search binds the query vector as $1, then calls with startArg=1.
	f, err := BuildFilter(metaCols(), nil, `{"source": {"eq": "handbook"}}`, 1)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).To(Equal(`"metadata"->>$2 = $3`),
		"with startArg=1 the first filter placeholder must be $2, not $1")

	// And Args must NOT include the caller's slots — the caller owns those.
	Expect(f.Args).To(Equal([]interface{}{"source", "handbook"}))
}

// Whatever the caller already bound, the first filter placeholder is
// $(startArg+1) and Args holds exactly the filter's own values.
func TestBuildFilter_StartArgTable(t *testing.T) {
	RegisterTestingT(t)

	for _, startArg := range []int{0, 1, 2, 5, 17} {
		t.Run(fmt.Sprintf("startArg=%d", startArg), func(t *testing.T) {
			RegisterTestingT(t)

			f, err := BuildFilter(metaCols(), nil,
				`{"a": {"eq": "1"}, "b": {"in": ["x","y"]}}`, startArg)
			Expect(err).ToNot(HaveOccurred())

			Expect(f.SQL).To(Equal(fmt.Sprintf(
				`"metadata"->>$%d = $%d AND "metadata"->>$%d = ANY($%d::text[])`,
				startArg+1, startArg+2, startArg+3, startArg+4)))

			// Exactly the filter's own args, in placeholder order, with no nil
			// padding for the caller's slots left in.
			Expect(f.Args).To(HaveLen(4))
			Expect(f.Args[0]).To(Equal("a"))
			Expect(f.Args[1]).To(Equal("1"))
			Expect(f.Args[2]).To(Equal("b"))
			Expect(f.Args[3]).To(Equal(pq.Array([]string{"x", "y"})))
			for i, a := range f.Args {
				Expect(a).ToNot(BeNil(), "arg %d is a leftover caller placeholder slot", i)
			}
		})
	}
}

// The simple grid and the advanced JSON share one $n counter, so a filter using
// both must not collide.
func TestBuildFilter_SimpleAndAdvancedShareTheCounter(t *testing.T) {
	RegisterTestingT(t)

	f, err := BuildFilter(metaCols(),
		kvFilter([]map[string]any{{"key": "source", "value": "handbook"}}),
		`{"page": {"gt": 3}}`, 1)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).To(Equal(`"metadata"->>$2 = $3 AND ("metadata"->>$4)::numeric > $5`))
	Expect(f.Args).To(Equal([]interface{}{"source", "handbook", "page", float64(3)}))
}

// ---------------------------------------------------------------------------
// The simple key/value surface
// ---------------------------------------------------------------------------

func TestBuildFilter_SimpleKeyValue(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name string
		val  any
	}{
		{
			// Straight from the editor: a KeyValueArray of maps.
			name: "[]any of maps",
			val: []any{
				map[string]any{"key": "source", "value": "handbook"},
				map[string]any{"key": "lang", "value": "en"},
			},
		},
		{
			name: "[]map[string]any",
			val: []map[string]any{
				{"key": "source", "value": "handbook"},
				{"key": "lang", "value": "en"},
			},
		},
		{
			// The ${...} substitution pass rewrites the whole value to its JSON
			// text, so the same grid can arrive as a string.
			name: "JSON string (the ${...} substituted form)",
			val:  `[{"key":"source","value":"handbook"},{"key":"lang","value":"en"}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)
			f, err := BuildFilter(metaCols(), kvFilter(tt.val), "", 0)
			Expect(err).ToNot(HaveOccurred())
			// AND-ed exact equality, in the order the operator entered them.
			Expect(f.SQL).To(Equal(`"metadata"->>$1 = $2 AND "metadata"->>$3 = $4`))
			Expect(f.Args).To(Equal([]interface{}{"source", "handbook", "lang", "en"}))
		})
	}
}

func TestBuildFilter_SimpleKeyValue_EdgeCases(t *testing.T) {
	RegisterTestingT(t)

	// A blank key is a half-filled row in the grid, not an error.
	f, err := BuildFilter(metaCols(),
		kvFilter([]map[string]any{{"key": "", "value": "x"}, {"key": "a", "value": "1"}}), "", 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).To(Equal(`"metadata"->>$1 = $2`))
	Expect(f.Args).To(Equal([]interface{}{"a", "1"}))

	// Non-string values render the way jsonb's ->> would.
	f, err = BuildFilter(metaCols(),
		kvFilter([]map[string]any{{"key": "page", "value": 3}, {"key": "ok", "value": true}}), "", 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.Args).To(Equal([]interface{}{"page", "3", "ok", "true"}))

	// Empty forms are all "no filter".
	for _, empty := range []any{nil, "", "[]", "null", []any{}} {
		f, err := BuildFilter(metaCols(), kvFilter(empty), "", 0)
		Expect(err).ToNot(HaveOccurred(), "value %#v", empty)
		Expect(f.SQL).To(Equal(""), "value %#v", empty)
	}

	// Garbage is worth naming.
	_, err = BuildFilter(metaCols(), kvFilter("not json"), "", 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("couldn't read the Metadata Filter"))
}

// ---------------------------------------------------------------------------
// Nothing to filter on
// ---------------------------------------------------------------------------

// An absent filter must produce an EMPTY SQL fragment so the caller omits the
// WHERE clause entirely — not "WHERE true", and certainly not "WHERE ".
func TestBuildFilter_EmptyIsEmpty(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name     string
		simple   []*core.Connection
		advanced string
	}{
		{"both absent", nil, ""},
		{"nil connection in the slice", []*core.Connection{nil}, ""},
		{"connection with nil value", kvFilter(nil), ""},
		{"empty advanced object", nil, "{}"},
		{"advanced null", nil, "null"},
		{"advanced whitespace", nil, "   "},
		{"empty grid and empty object", kvFilter("[]"), "{}"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			RegisterTestingT(t)
			f, err := BuildFilter(metaCols(), tt.simple, tt.advanced, 0)
			Expect(err).ToNot(HaveOccurred())
			Expect(f.SQL).To(Equal(""))
			Expect(f.Args).To(BeEmpty())
		})
	}
}

// A table with no metadata column cannot be filtered — and this must be caught
// before we generate SQL against a column that isn't there.
func TestBuildFilter_NoMetadataColumn(t *testing.T) {
	RegisterTestingT(t)

	_, err := BuildFilter(noMetaCols(), nil, `{"source": {"eq": "handbook"}}`, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("no metadata column"))
	Expect(err.Error()).To(ContainSubstring("remove the metadata filter"))

	_, err = BuildFilter(noMetaCols(),
		kvFilter([]map[string]any{{"key": "a", "value": "1"}}), "", 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("no metadata column"))

	// But NO filter against a table with no metadata column is perfectly fine.
	f, err := BuildFilter(noMetaCols(), nil, "", 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(f.SQL).To(Equal(""))
}

// ---------------------------------------------------------------------------
// Guards
// ---------------------------------------------------------------------------

func TestBuildFilter_DepthGuard(t *testing.T) {
	RegisterTestingT(t)

	// The top-level object is depth 1, so five nested groups reach depth 6.
	deep := `{"a": {"eq": "1"}}`
	for i := 0; i < 5; i++ {
		deep = `{"$and": [` + deep + `]}`
	}
	_, err := BuildFilter(metaCols(), nil, deep, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("nested more than 5 levels deep"))

	// One level shallower is fine.
	ok := `{"a": {"eq": "1"}}`
	for i := 0; i < 4; i++ {
		ok = `{"$and": [` + ok + `]}`
	}
	_, err = BuildFilter(metaCols(), nil, ok, 0)
	Expect(err).ToNot(HaveOccurred())
}

func TestBuildFilter_PredicateGuard(t *testing.T) {
	RegisterTestingT(t)

	build := func(n int) string {
		obj := map[string]any{}
		for i := 0; i < n; i++ {
			obj[fmt.Sprintf("f%03d", i)] = map[string]any{"eq": "x"}
		}
		b, err := json.Marshal(obj)
		Expect(err).ToNot(HaveOccurred())
		return string(b)
	}

	_, err := BuildFilter(metaCols(), nil, build(51), 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("more than 50 conditions"))

	f, err := BuildFilter(metaCols(), nil, build(50), 0)
	Expect(err).ToNot(HaveOccurred())
	Expect(strings.Count(f.SQL, " AND ")).To(Equal(49))
}

// !!! FAILING — REAL BUG IN THE FOUNDATION (actions/vectordatabase/pgvector/filter.go, BuildFilter) !!!
//
// The 50-predicate ceiling guards the advanced JSON surface but NOT the simple
// key/value grid, so the grid bypasses it completely: 500 pairs compile happily
// into 500 predicates and 1000 bound parameters, while 51 advanced fields are
// (correctly) rejected.
//
// The smoking gun that this is an oversight rather than a design decision is in
// BuildFilter itself — the key/value loop DOES bump the counter:
//
//	for _, pair := range kv {
//	    b.predicates++                       // <- counted...
//	    clauses = append(clauses, ...)       // <- ...but never checked
//	}
//
// The increment is dead code. Only compileField ever tests the ceiling, and the
// grid never reaches it.
//
// It matters because the grid is not limited to what a human can type into the
// UI: a KeyValueArray also arrives as JSON text through the ${...} substitution
// pass (TestBuildFilter_SimpleKeyValue pins that path), so an upstream step can
// hand this an arbitrarily large filter — which is precisely the unbounded query
// the guard was written to prevent. Postgres's wire protocol caps a statement at
// 65535 parameters, and this generates two per pair.
//
// Fix (in filter.go — deliberately NOT applied here): check the ceiling in the
// key/value loop as well, e.g.
//
//	for _, pair := range kv {
//	    b.predicates++
//	    if b.predicates > maxFilterPredicates {
//	        return Filter{}, fmt.Errorf("the Metadata Filter has more than %d conditions", maxFilterPredicates)
//	    }
//	    ...
//	}
func TestBuildFilter_PredicateGuardCoversTheKeyValueGridToo(t *testing.T) {
	RegisterTestingT(t)

	rows := make([]map[string]any, 500)
	for i := range rows {
		rows[i] = map[string]any{"key": fmt.Sprintf("k%03d", i), "value": "v"}
	}

	f, err := BuildFilter(metaCols(), kvFilter(rows), "", 0)

	Expect(err).To(HaveOccurred(),
		"a 500-pair key/value grid compiled into %d predicates and %d bound parameters "+
			"without tripping the %d-predicate guard — the guard only covers the advanced JSON surface",
		strings.Count(f.SQL, "->>"), len(f.Args), maxFilterPredicates)
	Expect(err.Error()).To(ContainSubstring("more than 50 conditions"))
}

func TestBuildFilter_UnknownOperator(t *testing.T) {
	RegisterTestingT(t)

	_, err := BuildFilter(metaCols(), nil, `{"page": {"greaterThan": 3}}`, 0)
	Expect(err).To(HaveOccurred())

	msg := err.Error()
	Expect(msg).To(ContainSubstring(`"greaterThan" isn't a filter operator`))
	// The message must name the ones that ARE valid.
	for _, op := range []string{
		"eq", "neq", "in", "notIn", "gt", "gte", "lt", "lte",
		"like", "ilike", "exists", "isNull", "arrayContains", "contains",
	} {
		Expect(msg).To(ContainSubstring(op), "the error must list %q as a valid operator", op)
	}
}

func TestBuildFilter_MalformedAdvancedJSON(t *testing.T) {
	RegisterTestingT(t)

	_, err := BuildFilter(metaCols(), nil, `{"source": `, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("isn't valid JSON"))
	// And it shows what good looks like.
	Expect(err.Error()).To(ContainSubstring(`{"source": {"eq": "handbook"}, "page": {"gt": 3}}`))
}

func TestBuildFilter_ExistsAndIsNullMustBeBoolean(t *testing.T) {
	RegisterTestingT(t)

	_, err := BuildFilter(metaCols(), nil, `{"tag": {"exists": "yes"}}`, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("tag.exists must be true or false"))

	_, err = BuildFilter(metaCols(), nil, `{"tag": {"isNull": "yes"}}`, 0)
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("tag.isNull must be true or false"))
}

// The advanced filter arrives as an object input, which the ${...} pass may have
// rewritten into its JSON text — either way it is a string by the time it gets
// here, and both forms must compile identically.
func TestBuildFilter_DeterministicOrdering(t *testing.T) {
	RegisterTestingT(t)

	// Go map iteration is random; the compiled SQL must not be.
	const advanced = `{"zeta": {"eq": "z"}, "alpha": {"eq": "a"}, "mid": {"eq": "m"}}`
	first, err := BuildFilter(metaCols(), nil, advanced, 0)
	Expect(err).ToNot(HaveOccurred())

	for i := 0; i < 20; i++ {
		f, err := BuildFilter(metaCols(), nil, advanced, 0)
		Expect(err).ToNot(HaveOccurred())
		Expect(f.SQL).To(Equal(first.SQL))
		Expect(f.Args).To(Equal(first.Args))
	}
	// Sorted: alpha, mid, zeta.
	Expect(first.Args).To(Equal([]interface{}{"alpha", "a", "mid", "m", "zeta", "z"}))
}
