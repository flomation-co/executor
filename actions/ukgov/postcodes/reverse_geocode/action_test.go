package ukgov_postcodes_reverse_geocode

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/postcodes"
	. "github.com/onsi/gomega"
)

func TestReverseGeocode(t *testing.T) {
	RegisterTestingT(t)
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":200,"result":[{"postcode":"SW1A 1AA","region":"London","admin_district":"Westminster","distance":12.3},{"postcode":"SW1A 2AA","region":"London","admin_district":"Westminster","distance":80.1}]}`))
	}))
	defer srv.Close()

	old := postcodes.BaseURL
	postcodes.BaseURL = srv.URL
	defer func() { postcodes.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "latitude", Type: core.ConnectionTypeString, Value: "51.501"},
		{Name: "longitude", Type: core.ConnectionTypeString, Value: "-0.1416"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(out["nearest"]).To(Equal("SW1A 1AA"))
	Expect(gotQuery).To(ContainSubstring("lon=-0.1416"))
	Expect(out["tool_result"]).To(ContainSubstring("SW1A 1AA"))
}

func TestReverseGeocodeNoResult(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":200,"result":null}`))
	}))
	defer srv.Close()

	old := postcodes.BaseURL
	postcodes.BaseURL = srv.URL
	defer func() { postcodes.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "latitude", Type: core.ConnectionTypeString, Value: "0"},
		{Name: "longitude", Type: core.ConnectionTypeString, Value: "0"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(0))
	Expect(out["tool_result"]).To(ContainSubstring("No UK postcodes found"))
}
