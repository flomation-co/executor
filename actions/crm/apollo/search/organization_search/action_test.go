package organization_search

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

// Company search filters must reach Apollo as URL query parameters (arrays with
// bracket notation), not the JSON body — otherwise Apollo ignores the location
// filter and returns a generic, unscoped company list.
func TestExecute_CompanyFiltersAreQueryParams(t *testing.T) {
	RegisterTestingT(t)

	var q url.Values
	cleanup := withServer(t, func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/mixed_companies/search"))
		q = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"organizations":[{"id":"o1","name":"Acme"}]}`))
	})
	defer cleanup()

	res, err := Execute(nil, nil, inputs(
		[2]string{"api_key", "k123"},
		[2]string{"q_organization_name", "Acme"},
		[2]string{"organization_locations", "Chester, Liverpool"},
	))
	Expect(err).ToNot(HaveOccurred())
	Expect(res["success"]).To(BeTrue())

	Expect(q.Get("q_organization_name")).To(Equal("Acme"))
	Expect(q["organization_locations[]"]).To(Equal([]string{"Chester", "Liverpool"}))
}
