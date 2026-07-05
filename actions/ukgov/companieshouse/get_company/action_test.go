package ukgov_companieshouse_get_company

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/companieshouse"
	. "github.com/onsi/gomega"
)

func TestGetCompany(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"company_name":"FLOMATION LTD","company_number":"12345678","company_status":"active","type":"ltd","date_of_creation":"2020-01-01","has_charges":true,"registered_office_address":{"address_line_1":"Ruscoe House","locality":"Whitchurch","postal_code":"SY13 2JJ","country":"Wales"}}`))
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
	Expect(gotPath).To(Equal("/company/12345678"))
	Expect(out["company_name"]).To(Equal("FLOMATION LTD"))
	Expect(out["company_status"]).To(Equal("active"))
	Expect(out["registered_office"]).To(ContainSubstring("SY13 2JJ"))
	Expect(out["tool_result"]).To(ContainSubstring("Incorporated 2020-01-01"))
	Expect(out["tool_result"]).To(ContainSubstring("has registered charges"))
}

func TestGetCompanyNotFound(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	old := companieshouse.BaseURL
	companieshouse.BaseURL = srv.URL
	defer func() { companieshouse.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
		{Name: "company_number", Type: core.ConnectionTypeString, Value: "00000000"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("No company found"))
}
