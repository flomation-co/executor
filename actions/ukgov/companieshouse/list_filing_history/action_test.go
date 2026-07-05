package ukgov_companieshouse_list_filing_history

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/companieshouse"
	. "github.com/onsi/gomega"
)

func TestListFilingHistory(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total_count":42,"items":[
		  {"transaction_id":"abc","category":"accounts","type":"AA","date":"2024-09-30","description":"Accounts made up to 31 December 2023"}
		]}`))
	}))
	defer srv.Close()

	old := companieshouse.BaseURL
	companieshouse.BaseURL = srv.URL
	defer func() { companieshouse.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
		{Name: "company_number", Type: core.ConnectionTypeString, Value: "12345678"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(out["total"]).To(Equal(42))
	Expect(gotPath).To(Equal("/company/12345678/filing-history"))
	Expect(out["tool_result"]).To(ContainSubstring("Latest: 2024-09-30"))
}
