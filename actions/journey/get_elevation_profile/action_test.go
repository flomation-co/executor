package journey_get_elevation_profile

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

const googleElevationOK = `{
  "status": "OK",
  "results": [
    {"elevation": 100, "resolution": 5, "location": {"lat": 51.5, "lng": -0.1}},
    {"elevation": 150, "resolution": 5, "location": {"lat": 51.6, "lng": -0.1}},
    {"elevation": 120, "resolution": 5, "location": {"lat": 51.7, "lng": -0.1}}
  ]
}`

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleElevationOK)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("polyline", "_p~iF~ps|U"),
		conn("sample_count", "3"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	// Deltas: +50, -30 → ascent 50, descent 30
	Expect(out["total_ascent_metres"]).To(Equal("50"))
	Expect(out["total_descent_metres"]).To(Equal("30"))
	Expect(out["min_elevation_metres"]).To(Equal("100"))
	Expect(out["max_elevation_metres"]).To(Equal("150"))
}

func TestExecuteMissingPolyline(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleElevationOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("polyline"))
}
