package query

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
	. "github.com/onsi/gomega"
)

func TestQuery(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotAuth, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotQuery = r.URL.Query().Get("query")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"QueryResponse":{"Invoice":[{"Id":"1","TotalAmt":150.0}],"maxResults":1}}`))
	}))
	defer srv.Close()

	old := quickbooks_common.ProductionBaseURL
	quickbooks_common.ProductionBaseURL = srv.URL
	defer func() { quickbooks_common.ProductionBaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok-abc"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "9130347"},
		{Name: "query", Type: core.ConnectionTypeText, Value: "select * from Invoice where TotalAmt > '100'"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(gotAuth).To(Equal("Bearer tok-abc"))
	Expect(gotPath).To(Equal("/v3/company/9130347/query"))
	Expect(gotQuery).To(Equal("select * from Invoice where TotalAmt > '100'"))

	result, ok := out["result"].(map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(result).To(HaveKey("QueryResponse"))
}

func TestQueryMissingSQL(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok"},
		{Name: "company", Type: core.ConnectionTypeString, Value: "1"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(strings.ToLower(out["error"].(string))).To(ContainSubstring("query is required"))
}
