package journey_compare_departure_times

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	journey "flomation.app/automate/executor/actions/journey"
	. "github.com/onsi/gomega"
)

type stubTransport struct {
	calls  int
	bodies []string
}

func (s *stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	body := s.bodies[s.calls%len(s.bodies)]
	s.calls++
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Header:     make(http.Header),
	}, nil
}

func swapClient(bodies []string) (func(), *stubTransport) {
	tr := &stubTransport{bodies: bodies}
	prev := journey.DefaultClient
	journey.DefaultClient = &http.Client{Transport: tr, Timeout: time.Second}
	return func() { journey.DefaultClient = prev }, tr
}

func conn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func routeBody(durationSecs int) string {
	return `{
  "status": "OK",
  "routes": [{
    "overview_polyline": {"points": "p"},
    "bounds": {"northeast": {"lat": 53, "lng": -2}, "southwest": {"lat": 51, "lng": -2}},
    "legs": [{"distance": {"value": 100000}, "duration": {"value": ` + strconv.Itoa(durationSecs) + `}, "steps": []}]
  }]
}`
}

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)
	// Three calls; second is slowest (rush hour)
	cleanup, tr := swapClient([]string{
		routeBody(3600),
		routeBody(7200),
		routeBody(3000),
	})
	defer cleanup()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("origin", "London"),
		conn("destination", "Manchester"),
		conn("departure_times", "2026-06-12T07:00:00Z,2026-06-12T08:00:00Z,2026-06-12T17:00:00Z"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["best_duration_friendly"]).To(Equal("50 minutes"))
	Expect(out["worst_duration_friendly"]).To(Equal("2 hours"))
	Expect(out["delta_seconds"]).To(Equal("4200"))
	Expect(tr.calls).To(Equal(3))
}

func TestExecuteInvalidTime(t *testing.T) {
	RegisterTestingT(t)
	cleanup, _ := swapClient([]string{routeBody(3600)})
	defer cleanup()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("origin", "A"),
		conn("destination", "B"),
		conn("departure_times", "not-a-timestamp"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("departure_times"))
}
