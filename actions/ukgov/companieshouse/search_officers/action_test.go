package ukgov_companieshouse_search_officers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/companieshouse"
	. "github.com/onsi/gomega"
)

func TestSearchOfficers(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total_results":1,"items":[{"title":"John SMITH","description":"Born 1970","appointment_count":3}]}`))
	}))
	defer srv.Close()

	old := companieshouse.BaseURL
	companieshouse.BaseURL = srv.URL
	defer func() { companieshouse.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
		{Name: "query", Type: core.ConnectionTypeString, Value: "John Smith"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(gotPath).To(Equal("/search/officers"))
	Expect(out["tool_result"]).To(ContainSubstring("John SMITH (3 appointment(s))"))
}
