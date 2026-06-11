package journey_calculate_route

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

const googleDirectionsOK = `{
  "status": "OK",
  "routes": [{
    "overview_polyline": {"points": "encoded"},
    "bounds": {"northeast": {"lat": 53.5, "lng": -2.2}, "southwest": {"lat": 51.5, "lng": -2.3}},
    "legs": [{
      "distance": {"value": 322000},
      "duration": {"value": 13500},
      "steps": [{"html_instructions": "Head north", "distance": {"value": 500}, "duration": {"value": 60}}]
    }]
  }]
}`

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleDirectionsOK)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("origin", "London"),
		conn("destination", "Manchester"),
		conn("mode", "driving"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["distance_miles"]).To(Equal("200.08"))
	Expect(out["duration_friendly"]).To(Equal("3 hours 45 minutes"))
	Expect(out["polyline"]).To(Equal("encoded"))
	Expect(out["tool_result"]).To(ContainSubstring("London"))
	Expect(out["tool_result"]).To(ContainSubstring("Manchester"))
}

func TestExecuteWaypointsParsed(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleDirectionsOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("origin", "London"),
		conn("destination", "Manchester"),
		conn("waypoints", "Birmingham|Stoke"),
	})
	Expect(err).ToNot(HaveOccurred())
}

func TestExecuteMissingDestination(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleDirectionsOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("origin", "London"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("destination"))
}

func TestExecuteInvalidDepartTime(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleDirectionsOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("origin", "London"),
		conn("destination", "Manchester"),
		conn("depart_at", "not-a-date"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("depart_at"))
}
