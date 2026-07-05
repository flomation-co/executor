package ukgov_environmentagency_station_readings

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/environmentagency"
	. "github.com/onsi/gomega"
)

func TestStationReadings(t *testing.T) {
	RegisterTestingT(t)
	var gotRawURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRawURL = r.URL.RequestURI()
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[
		  {"parameter":"level","parameterName":"Water Level","qualifier":"Stage","unitName":"mASD","latestReading":{"value":1.234,"dateTime":"2024-01-01T09:00:00Z"}},
		  {"parameter":"rainfall","parameterName":"Rainfall","qualifier":"","unitName":"mm","latestReading":{"value":0.2,"dateTime":"2024-01-01T09:00:00Z"}}
		]}`))
	}))
	defer srv.Close()

	old := environmentagency.BaseURL
	environmentagency.BaseURL = srv.URL
	defer func() { environmentagency.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "station_reference", Type: core.ConnectionTypeString, Value: "2001"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	// The `latest` flag must be present in the request URI.
	Expect(gotRawURL).To(ContainSubstring("stationReference=2001"))
	Expect(gotRawURL).To(ContainSubstring("latest"))
	Expect(out["tool_result"]).To(ContainSubstring("Water Level (Stage) 1.234 mASD"))
	Expect(out["tool_result"]).To(ContainSubstring("Rainfall 0.2 mm"))
}

func TestStationReadingsNoStation(t *testing.T) {
	RegisterTestingT(t)
	// Unknown station reference returns HTTP 200 with an empty items array.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	old := environmentagency.BaseURL
	environmentagency.BaseURL = srv.URL
	defer func() { environmentagency.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "station_reference", Type: core.ConnectionTypeString, Value: "BOGUS"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("No readings found for station"))
}
