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
		[2]string{"locations", "Chester, Liverpool"},
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
