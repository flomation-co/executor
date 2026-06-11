package journey_find_nearby_places

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

const googleNearbyOK = `{
  "status": "OK",
  "results": [{
    "place_id": "ChIJ1",
    "name": "Test Cafe",
    "vicinity": "1 Test St",
    "geometry": {"location": {"lat": 51.5, "lng": -0.12}},
    "rating": 4.5
  }]
}`

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleNearbyOK)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("latitude", "51.5"),
		conn("longitude", "-0.12"),
		conn("category", "cafe"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal("1"))
	Expect(out["tool_result"]).To(ContainSubstring("Test Cafe"))
}

func TestExecuteInvalidLatitude(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleNearbyOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("latitude", "not-a-number"),
		conn("longitude", "-0.12"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("latitude"))
}
