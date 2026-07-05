package ukgov_companieshouse_search_companies

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/companieshouse"
	. "github.com/onsi/gomega"
)

func TestSearchCompanies(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotQuery, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total_results":2,"items":[
		  {"company_number":"12345678","title":"FLOMATION LTD","company_status":"active","company_type":"ltd"},
		  {"company_number":"87654321","title":"FLOMATION HOLDINGS LTD","company_status":"dissolved"}
		]}`))
	}))
	defer srv.Close()

	old := companieshouse.BaseURL
	companieshouse.BaseURL = srv.URL
	defer func() { companieshouse.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
		{Name: "query", Type: core.ConnectionTypeString, Value: "Flomation"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(out["total"]).To(Equal(2))
	Expect(gotPath).To(Equal("/search/companies"))
	Expect(gotQuery).To(ContainSubstring("q=Flomation"))
	// Basic auth: base64("testkey:") = dGVzdGtleTo=
	Expect(gotAuth).To(Equal("Basic dGVzdGtleTo="))
	Expect(out["tool_result"]).To(ContainSubstring("FLOMATION LTD (12345678, active)"))
}

func TestSearchCompaniesAuthError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	old := companieshouse.BaseURL
	companieshouse.BaseURL = srv.URL
	defer func() { companieshouse.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "bad"},
		{Name: "query", Type: core.ConnectionTypeString, Value: "x"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("authentication failed"))
}

func TestSearchCompaniesRequiresKey(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "query", Type: core.ConnectionTypeString, Value: "x"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("API key is required"))
}
