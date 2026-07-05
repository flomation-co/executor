package ukgov_police_street_crimes

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/police"
	. "github.com/onsi/gomega"
)

const sample = `[
  {"category":"anti-social-behaviour","location":{"latitude":"52.6","longitude":"-1.1","street":{"id":1,"name":"On or near High St"}},"month":"2024-01","outcome_status":null,"id":1},
  {"category":"anti-social-behaviour","location":{"latitude":"52.6","longitude":"-1.1","street":{"id":1,"name":"On or near High St"}},"month":"2024-01","id":2},
  {"category":"violent-crime","location":{"latitude":"52.6","longitude":"-1.1","street":{"id":2,"name":"On or near Low St"}},"month":"2024-01","outcome_status":{"category":"Unable to prosecute suspect","date":"2024-02"},"id":3}
]`

func TestStreetCrimes(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	old := police.BaseURL
	police.BaseURL = srv.URL
	defer func() { police.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "latitude", Type: core.ConnectionTypeString, Value: "52.6"},
		{Name: "longitude", Type: core.ConnectionTypeString, Value: "-1.1"},
		{Name: "date", Type: core.ConnectionTypeString, Value: "2024-01"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(3))
	Expect(gotPath).To(Equal("/crimes-street/all-crime"))
	Expect(gotQuery).To(ContainSubstring("lat=52.6"))
	Expect(gotQuery).To(ContainSubstring("date=2024-01"))
	// anti-social-behaviour (2) should lead violent-crime (1).
	Expect(out["tool_result"]).To(ContainSubstring("anti-social-behaviour (2)"))
}

func TestStreetCrimesRequiresCoords(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "latitude", Type: core.ConnectionTypeString, Value: "52.6"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("longitude is required"))
}
