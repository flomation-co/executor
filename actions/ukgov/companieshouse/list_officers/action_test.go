package ukgov_companieshouse_list_officers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/companieshouse"
	. "github.com/onsi/gomega"
)

func TestListOfficers(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total_results":2,"active_count":1,"resigned_count":1,"items":[
		  {"name":"SMITH, John","officer_role":"director","appointed_on":"2020-01-01"},
		  {"name":"JONES, Jane","officer_role":"secretary","appointed_on":"2019-01-01","resigned_on":"2021-06-01"}
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
	Expect(out["count"]).To(Equal(2))
	Expect(out["active_count"]).To(Equal(1))
	Expect(gotPath).To(Equal("/company/12345678/officers"))
	Expect(out["tool_result"]).To(ContainSubstring("SMITH, John (director, active)"))
	Expect(out["tool_result"]).To(ContainSubstring("JONES, Jane (secretary, resigned)"))
}
