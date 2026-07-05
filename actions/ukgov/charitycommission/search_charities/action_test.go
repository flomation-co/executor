package ukgov_charitycommission_search_charities

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/charitycommission"
	. "github.com/onsi/gomega"
)

func TestSearchCharities(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotKey string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("Ocp-Apim-Subscription-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[
		  {"organisation_number":123,"reg_charity_number":202918,"charity_name":"OXFAM","reg_status":"Registered","date_of_registration":"1963-03-27"}
		]`))
	}))
	defer srv.Close()

	old := charitycommission.BaseURL
	charitycommission.BaseURL = srv.URL
	defer func() { charitycommission.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
		{Name: "name", Type: core.ConnectionTypeString, Value: "Oxfam"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(gotPath).To(Equal("/searchCharityName/Oxfam"))
	Expect(gotKey).To(Equal("testkey"))
	Expect(out["tool_result"]).To(ContainSubstring("OXFAM (202918, Registered)"))
}

func TestSearchCharitiesAuthError(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	old := charitycommission.BaseURL
	charitycommission.BaseURL = srv.URL
	defer func() { charitycommission.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "bad"},
		{Name: "name", Type: core.ConnectionTypeString, Value: "Oxfam"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("authentication failed"))
}
