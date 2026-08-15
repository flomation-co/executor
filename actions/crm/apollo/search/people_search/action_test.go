package people_search

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
	. "github.com/onsi/gomega"
)

func inputs(pairs ...[2]string) []*core.Connection {
	out := make([]*core.Connection, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, &core.Connection{Name: p[0], Type: core.ConnectionTypeString, Value: p[1]})
	}
	return out
}

func withServer(t *testing.T, handler http.HandlerFunc) func() {
	t.Helper()
	srv := httptest.NewServer(handler)
	orig := apollo_common.BaseURL
	apollo_common.BaseURL = srv.URL
	return func() {
		apollo_common.BaseURL = orig
		srv.Close()
	}
}

// Every filter must reach Apollo as a URL query parameter (array filters with
// bracket notation), NOT in the JSON body — otherwise Apollo silently ignores
// the location/domain/title filters and returns a generic list of the wrong
// people (the reported bug).
func TestExecute_FiltersAreQueryParamsWithBracketArrays(t *testing.T) {
	RegisterTestingT(t)

	var q url.Values
	cleanup := withServer(t, func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/mixed_people/api_search"))
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"people":[{"id":"p_1","name":"A Person"}]}`))
	})
	defer cleanup()

	ins := inputs(
		[2]string{"api_key", "k123"},
		[2]string{"person_titles", "Chief Executive, Founder"},
		// Semicolon-separated: a comma belongs inside one location value.
		[2]string{"person_locations", "Chester, United Kingdom; Wrexham, United Kingdom"},
		[2]string{"organization_domains", "https://www.HSE.ie/about"},
	)
	ins = append(ins,
		&core.Connection{Name: "page", Type: core.ConnectionTypeInteger, Value: "2"},
		&core.Connection{Name: "per_page", Type: core.ConnectionTypeInteger, Value: "10"},
	)
	res, err := Execute(nil, nil, ins)
	Expect(err).ToNot(HaveOccurred())
	Expect(res["success"]).To(BeTrue())

	// Arrays as repeated bracketed query params.
	Expect(q["person_titles[]"]).To(Equal([]string{"Chief Executive", "Founder"}))
	// Each location stays whole; the country must never become its own value,
	// which would OR the city away and widen the search to the entire country.
	Expect(q["person_locations[]"]).To(Equal([]string{"Chester, United Kingdom", "Wrexham, United Kingdom"}))
	Expect(q["person_locations[]"]).ToNot(ContainElement("United Kingdom"))
	Expect(q["q_organization_domains_list[]"]).To(Equal([]string{"hse.ie"}))
	// Scalars.
	Expect(q.Get("page")).To(Equal("2"))
	Expect(q.Get("per_page")).To(Equal("10"))
}

func TestExecute_OmitsAbsentFilters(t *testing.T) {
	RegisterTestingT(t)

	var q url.Values
	cleanup := withServer(t, func(w http.ResponseWriter, r *http.Request) {
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"people":[]}`))
	})
	defer cleanup()

	_, err := Execute(nil, nil, inputs(
		[2]string{"api_key", "k123"},
		[2]string{"person_titles", "CEO"},
	))
	Expect(err).ToNot(HaveOccurred())
	Expect(q).ToNot(HaveKey("person_locations[]"))
	Expect(q).ToNot(HaveKey("q_organization_domains_list[]"))
}

func TestNormaliseDomains(t *testing.T) {
	RegisterTestingT(t)

	Expect(normaliseDomains([]string{"https://www.HSE.ie/about"})).To(Equal([]string{"hse.ie"}))
	Expect(normaliseDomains([]string{"jo@nice.org.uk"})).To(Equal([]string{"nice.org.uk"}))
	Expect(normaliseDomains([]string{" Example.COM ", "acme.io"})).To(Equal([]string{"example.com", "acme.io"}))
	Expect(normaliseDomains([]string{"notadomain", ""})).To(BeEmpty())
}
