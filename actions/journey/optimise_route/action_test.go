package journey_optimise_route

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
	. "github.com/onsi/gomega"
)

type stubTransport struct{ body string }

func (s *stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(s.body)),
		Header:     make(http.Header),
	}, nil
}

func swapClient(body string) func() {
	prev := journey.DefaultClient
	journey.DefaultClient = &http.Client{Transport: &stubTransport{body: body}, Timeout: time.Second}
	return func() { journey.DefaultClient = prev }
}

func conn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

const googleOptimiseOK = `{
  "status": "OK",
  "routes": [{
    "waypoint_order": [1, 0],
    "overview_polyline": {"points": "abc"},
    "bounds": {"northeast": {"lat": 53, "lng": -2}, "southwest": {"lat": 51, "lng": -2}},
    "legs": [
      {"distance": {"value": 50000}, "duration": {"value": 2400}, "steps": []},
      {"distance": {"value": 60000}, "duration": {"value": 2700}, "steps": []},
      {"distance": {"value": 70000}, "duration": {"value": 3000}, "steps": []}
    ]
  }]
}`

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleOptimiseOK)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("start", "London"),
		conn("end", "Manchester"),
		conn("stops", "Birmingham,Coventry"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["total_distance_miles"]).ToNot(BeEmpty())
	Expect(out["tool_result"]).To(ContainSubstring("Optimised 2 stops"))
}

func TestExecuteMissingStops(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleOptimiseOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("stops"))
}
