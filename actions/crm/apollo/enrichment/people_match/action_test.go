package people_match

import (
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

func TestExecute_Success(t *testing.T) {
	RegisterTestingT(t)

	var gotBody []byte
	cleanup := withServer(t, func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/people/match"))
		Expect(r.Header.Get("X-Api-Key")).To(Equal("k123"))
		gotBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"person":{"id":"p_1","name":"Ada Lovelace"}}`))
	})
	defer cleanup()

	res, err := Execute(nil, nil, inputs(
		[2]string{"api_key", "k123"},
		[2]string{"email", "ada@example.com"},
	))
	Expect(err).ToNot(HaveOccurred())
	Expect(res["success"]).To(BeTrue())
	Expect(res["id"]).To(Equal("p_1"))
	// tool_result carries the summary AND the enriched record (AI callers only
	// see tool_result).
	Expect(res["tool_result"].(string)).To(HavePrefix("Enriched Ada Lovelace"))
	Expect(res["tool_result"].(string)).To(ContainSubstring("Ada Lovelace"))
	Expect(string(gotBody)).To(ContainSubstring("ada@example.com"))
}

func TestExecute_NoMatch(t *testing.T) {
	RegisterTestingT(t)

	cleanup := withServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"person":null}`))
	})
	defer cleanup()

	res, err := Execute(nil, nil, inputs([2]string{"api_key", "k123"}, [2]string{"email", "nobody@example.com"}))
	Expect(err).ToNot(HaveOccurred())
	Expect(res["success"]).To(BeFalse())
	Expect(res["error"]).To(ContainSubstring("no matching person"))
}

func TestExecute_APIError(t *testing.T) {
	RegisterTestingT(t)

	cleanup := withServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limit exceeded"}`))
	})
	defer cleanup()

	// A 429 must be a graceful success=false (not a node error) so a flow can
	// branch on it.
	res, err := Execute(nil, nil, inputs([2]string{"api_key", "k123"}, [2]string{"email", "a@b.com"}))
	Expect(err).ToNot(HaveOccurred())
	Expect(res["success"]).To(BeFalse())
	Expect(res["error"]).To(Equal("rate limit exceeded"))
}

func TestExecute_MissingKey(t *testing.T) {
	RegisterTestingT(t)

	res, err := Execute(nil, nil, inputs([2]string{"email", "a@b.com"}))
	Expect(err).ToNot(HaveOccurred())
	Expect(res["success"]).To(BeFalse())
	Expect(res["error"]).To(ContainSubstring("API key"))
}
