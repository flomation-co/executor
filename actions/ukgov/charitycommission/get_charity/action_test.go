package ukgov_charitycommission_get_charity

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/charitycommission"
	. "github.com/onsi/gomega"
)

func TestGetCharity(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"charity_name":"OXFAM","reg_status":"Registered","charity_registration_number":202918}`))
	}))
	defer srv.Close()

	old := charitycommission.BaseURL
	charitycommission.BaseURL = srv.URL
	defer func() { charitycommission.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
		{Name: "charity_number", Type: core.ConnectionTypeString, Value: "202918"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(gotPath).To(Equal("/allcharitydetailsV2/202918/0")) // default suffix 0
	Expect(out["charity_name"]).To(Equal("OXFAM"))
	Expect(out["status"]).To(Equal("Registered"))
	Expect(out["tool_result"]).To(ContainSubstring("OXFAM (202918) — Registered"))
}

func TestGetCharityNotFound(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	old := charitycommission.BaseURL
	charitycommission.BaseURL = srv.URL
	defer func() { charitycommission.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
		{Name: "charity_number", Type: core.ConnectionTypeString, Value: "0"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("No charity found"))
}
