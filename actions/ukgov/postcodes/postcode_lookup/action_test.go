package ukgov_postcodes_postcode_lookup

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/postcodes"
	. "github.com/onsi/gomega"
)

func TestPostcodeLookup(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.EscapedPath()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":200,"result":{"postcode":"SW1A 1AA","longitude":-0.141588,"latitude":51.501009,"region":"London","country":"England","admin_district":"Westminster"}}`))
	}))
	defer srv.Close()

	old := postcodes.BaseURL
	postcodes.BaseURL = srv.URL
	defer func() { postcodes.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "postcode", Type: core.ConnectionTypeString, Value: "SW1A 1AA"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(gotPath).To(Equal("/postcodes/SW1A%201AA"))
	Expect(out["region"]).To(Equal("London"))
	Expect(out["tool_result"]).To(ContainSubstring("Westminster"))
}

func TestPostcodeLookupNotFound(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"status":404,"error":"Invalid postcode"}`))
	}))
	defer srv.Close()

	old := postcodes.BaseURL
	postcodes.BaseURL = srv.URL
	defer func() { postcodes.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "postcode", Type: core.ConnectionTypeString, Value: "ZZ1 1ZZ"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("not a recognised UK postcode"))
}
