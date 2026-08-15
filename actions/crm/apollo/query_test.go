package apollo_common

import (
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func qInputs(pairs ...[2]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, &core.Connection{Name: p[0], Type: core.ConnectionTypeString, Value: p[1]})
	}
	return out
}

func TestQueryBuilders(t *testing.T) {
	RegisterTestingT(t)

	ins := qInputs(
		[2]string{"name", "Acme"},
		// Generic comma-splitting stays for genuine comma lists (ids, domains).
		// Locations do NOT use this builder — see TestLocationList.
		[2]string{"locations", "Chester,Liverpool"},
		[2]string{"blank", "   "},
	)
	ins = append(ins,
		&core.Connection{Name: "page", Type: core.ConnectionTypeInteger, Value: "3"},
		&core.Connection{Name: "asc", Type: core.ConnectionTypeBoolean, Value: true},
	)

	q := url.Values{}
	AddQueryString(q, "q_name", "name", ins)
	AddQueryString(q, "q_blank", "blank", ins) // whitespace-only → omitted
	AddQueryList(q, "locations", "locations", ins)
	AddQueryInt(q, "page", "page", ins)
	AddQueryBool(q, "sort_ascending", "asc", ins)

	Expect(q.Get("q_name")).To(Equal("Acme"))
	Expect(q).ToNot(HaveKey("q_blank"))
	// Arrays use bracket notation, one entry per value.
	Expect(q["locations[]"]).To(Equal([]string{"Chester", "Liverpool"}))
	Expect(q.Get("page")).To(Equal("3"))
	Expect(q.Get("sort_ascending")).To(Equal("true"))
}

// A comma belongs INSIDE one Apollo location value ("Chester, United Kingdom").
// Splitting on it produced "Chester" OR "United Kingdom", and since Apollo ORs
// array filters the country clause swallowed the city — the search silently
// widened to the whole country while appearing to be scoped to a town.
func TestLocationList_DoesNotSplitOnCommas(t *testing.T) {
	RegisterTestingT(t)

	ins := qInputs([2]string{"loc", "Chester, United Kingdom"})
	Expect(LocationList("loc", ins)).To(Equal([]string{"Chester, United Kingdom"}))

	q := url.Values{}
	AddQueryLocationList(q, "organization_locations", "loc", ins)
	Expect(q["organization_locations[]"]).To(Equal([]string{"Chester, United Kingdom"}))
	// The country must never appear as a location of its own.
	Expect(q["organization_locations[]"]).ToNot(ContainElement("United Kingdom"))
}

// The failure this guards against is two DIFFERENT queries collapsing to the
// same thing: under comma-splitting both of these reduced to
// "<somewhere> OR United Kingdom" and returned an identical national list.
func TestLocationList_DistinctQueriesStayDistinct(t *testing.T) {
	RegisterTestingT(t)

	chester := LocationList("loc", qInputs([2]string{"loc", "Chester, United Kingdom"}))
	cheshire := LocationList("loc", qInputs([2]string{"loc", "Cheshire, United Kingdom"}))

	Expect(chester).ToNot(Equal(cheshire))
	Expect(chester).To(HaveLen(1))
	Expect(cheshire).To(HaveLen(1))
}

func TestLocationList_SemicolonSeparatesMultiple(t *testing.T) {
	RegisterTestingT(t)

	ins := qInputs([2]string{"loc", "Chester, United Kingdom; Wrexham, United Kingdom ;; Deeside, United Kingdom"})
	Expect(LocationList("loc", ins)).To(Equal([]string{
		"Chester, United Kingdom",
		"Wrexham, United Kingdom",
		"Deeside, United Kingdom",
	}))

	// Newlines work too, so a pasted column of locations behaves.
	nl := qInputs([2]string{"loc", "Chester, United Kingdom\nWrexham, United Kingdom"})
	Expect(LocationList("loc", nl)).To(HaveLen(2))
}

func TestLocationList_BlankAndAbsent(t *testing.T) {
	RegisterTestingT(t)

	Expect(LocationList("loc", qInputs([2]string{"loc", "   "}))).To(BeNil())
	Expect(LocationList("missing", qInputs())).To(BeNil())

	q := url.Values{}
	AddQueryLocationList(q, "organization_locations", "missing", qInputs())
	Expect(q).ToNot(HaveKey("organization_locations[]"))
}

func TestAddQueryFromMap(t *testing.T) {
	RegisterTestingT(t)

	q := url.Values{}
	AddQueryFromMap(q, map[string]interface{}{
		"q_keywords":  "cto",
		"stage_ids":   []interface{}{"a", "b"},
		"flag":        true,
		"per_page":    float64(50),
		"ignored_nil": nil,
	})

	Expect(q.Get("q_keywords")).To(Equal("cto"))
	Expect(q["stage_ids[]"]).To(Equal([]string{"a", "b"}))
	Expect(q.Get("flag")).To(Equal("true"))
	Expect(q.Get("per_page")).To(Equal("50"))
	Expect(q).ToNot(HaveKey("ignored_nil"))
}
