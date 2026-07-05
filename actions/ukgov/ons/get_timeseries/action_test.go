package ukgov_ons_get_timeseries

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

const sample = `{
  "description": {"title":"CPIH ANNUAL RATE 00: ALL ITEMS 2015=100","cdid":"L55O","unit":"%","preUnit":"","date":"2026 MAY","number":"3.0","datasetId":"MM23","nextRelease":"22 July 2026"},
  "months": [
    {"date":"2026 MAR","value":"2.8","year":"2026","month":"March"},
    {"date":"2026 APR","value":"2.9","year":"2026","month":"April"},
    {"date":"2026 MAY","value":"3.0","year":"2026","month":"May"}
  ]
}`

func TestGetTimeseries(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "cdid", Type: core.ConnectionTypeString, Value: "L55O"},
		{Name: "dataset", Type: core.ConnectionTypeString, Value: "mm23"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	// CDID and dataset lower-cased in the path; default section applied.
	Expect(gotPath).To(Equal("/economy/inflationandpriceindices/timeseries/l55o/mm23/data"))
	Expect(out["latest_value"]).To(Equal("3.0%"))
	Expect(out["latest_period"]).To(Equal("2026 MAY"))
	Expect(out["tool_result"]).To(ContainSubstring("latest 3.0% (2026 MAY)"))
	Expect(out["tool_result"]).To(ContainSubstring("Next release: 22 July 2026"))
}

func TestGetTimeseriesCustomSection(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	_, err := Execute(nil, nil, []*core.Connection{
		{Name: "cdid", Type: core.ConnectionTypeString, Value: "MGSX"},
		{Name: "dataset", Type: core.ConnectionTypeString, Value: "lms"},
		{Name: "section", Type: core.ConnectionTypeString, Value: "/employmentandlabourmarket/peopleinwork/"},
	})
	Expect(err).To(BeNil())
	Expect(gotPath).To(Equal("/employmentandlabourmarket/peopleinwork/timeseries/mgsx/lms/data"))
}

func TestGetTimeseriesRequiresCDID(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "dataset", Type: core.ConnectionTypeString, Value: "mm23"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("series ID (CDID) is required"))
}
