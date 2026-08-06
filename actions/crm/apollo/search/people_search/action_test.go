package people_search

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
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

// The domain filter must reach Apollo under q_organization_domains_list as bare,
// normalised domains — NOT the old organization_domains key, which Apollo
// silently ignored (returning a generic title-matched list of the wrong people).
func TestExecute_DomainFilterUsesCorrectKeyAndNormalises(t *testing.T) {
	RegisterTestingT(t)

	var gotBody []byte
	cleanup := withServer(t, func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/mixed_people/api_search"))
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"people":[{"id":"p_1","name":"A Person"}]}`))
	})
	defer cleanup()

	res, err := Execute(nil, nil, inputs(
		[2]string{"api_key", "k123"},
		[2]string{"person_titles", "Chief Executive"},
		// Messy input: scheme, www., uppercase, path, trailing spaces, and a
		// second bare domain.
		[2]string{"organization_domains", "https://www.HSE.ie/about , nice.org.uk "},
	))
	Expect(err).ToNot(HaveOccurred())
	Expect(res["success"]).To(BeTrue())

	var body map[string]interface{}
	Expect(json.Unmarshal(gotBody, &body)).To(Succeed())

	// Correct key present with normalised bare domains.
	Expect(body).To(HaveKey("q_organization_domains_list"))
	list := body["q_organization_domains_list"].([]interface{})
	Expect(list).To(Equal([]interface{}{"hse.ie", "nice.org.uk"}))

	// The old, ignored key must NOT be sent.
	Expect(body).ToNot(HaveKey("organization_domains"))
}

func TestExecute_NoDomainFilterOmitsKey(t *testing.T) {
	RegisterTestingT(t)

	var gotBody []byte
	cleanup := withServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"people":[]}`))
	})
	defer cleanup()

	_, err := Execute(nil, nil, inputs(
		[2]string{"api_key", "k123"},
		[2]string{"person_titles", "CEO"},
	))
	Expect(err).ToNot(HaveOccurred())

	var body map[string]interface{}
	Expect(json.Unmarshal(gotBody, &body)).To(Succeed())
	Expect(body).ToNot(HaveKey("q_organization_domains_list"))
}

func TestNormaliseDomains(t *testing.T) {
	RegisterTestingT(t)

	Expect(normaliseDomains([]string{"https://www.HSE.ie/about"})).To(Equal([]string{"hse.ie"}))
	Expect(normaliseDomains([]string{"jo@nice.org.uk"})).To(Equal([]string{"nice.org.uk"}))
	Expect(normaliseDomains([]string{" Example.COM ", "acme.io"})).To(Equal([]string{"example.com", "acme.io"}))
	// Entries without a dot (not a domain) are dropped so they can't widen the search.
	Expect(normaliseDomains([]string{"notadomain", ""})).To(BeEmpty())
}
