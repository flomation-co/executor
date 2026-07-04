package devops_jenkins_build_get_all

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func base(url string) []*core.Connection {
	return []*core.Connection{
		{Name: "base_url", Type: core.ConnectionTypeString, Value: url},
		{Name: "username", Type: core.ConnectionTypeString, Value: "admin"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "job", Type: core.ConnectionTypeString, Value: "deploy"},
	}
}

func TestListBuildsAppliesLimitRange(t *testing.T) {
	RegisterTestingT(t)
	var tree string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/job/deploy/api/json"))
		tree = r.URL.Query().Get("tree")
		_, _ = w.Write([]byte(`{"builds":[{"number":3},{"number":2}]}`))
	}))
	defer srv.Close()

	// Default (no return_all): capped page with a {0,limit} range.
	out, err := Execute(nil, nil, append(base(srv.URL),
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: int64(5)}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(tree).To(ContainSubstring("{0,5}"))
}

func TestListBuildsReturnAllUsesAllBuilds(t *testing.T) {
	RegisterTestingT(t)
	var tree string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tree = r.URL.Query().Get("tree")
		// Full history comes back under "allBuilds", not "builds".
		_, _ = w.Write([]byte(`{"allBuilds":[{"number":9},{"number":8},{"number":7}]}`))
	}))
	defer srv.Close()

	out, err := Execute(nil, nil, append(base(srv.URL),
		&core.Connection{Name: "return_all", Type: core.ConnectionTypeBoolean, Value: true}))
	Expect(err).To(BeNil())
	Expect(out["count"]).To(Equal(3)) // read from allBuilds, not the empty builds
	Expect(tree).To(ContainSubstring("allBuilds"))
	Expect(tree).To(ContainSubstring("{0,1000}")) // capped even in return-all mode
}

// A "Limit" wired to a whole-value reference that resolves to a non-numeric
// value (array/bool/nil) must be ignored, not panic the executor.
func TestListBuildsLimitNonNumericDoesNotPanic(t *testing.T) {
	RegisterTestingT(t)
	var tree string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tree = r.URL.Query().Get("tree")
		_, _ = w.Write([]byte(`{"builds":[{"number":1}]}`))
	}))
	defer srv.Close()

	out, err := Execute(nil, nil, append(base(srv.URL),
		&core.Connection{Name: "limit", Type: core.ConnectionTypeInteger, Value: []interface{}{"oops"}}))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(tree).To(ContainSubstring("{0,50}")) // falls back to the default limit
}
