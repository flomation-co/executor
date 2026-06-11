package journey_reverse_geocode

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

const googleReverseOK = `{
  "status": "OK",
  "results": [{
    "formatted_address": "Downing St, London SW1A 2AA, UK",
    "address_components": [
      {"long_name": "Downing Street", "types": ["route"]},
      {"long_name": "London", "types": ["postal_town"]},
      {"long_name": "United Kingdom", "types": ["country"]},
      {"long_name": "SW1A 2AA", "types": ["postal_code"]}
    ],
    "geometry": {"location": {"lat": 51.5, "lng": -0.12}, "location_type": "ROOFTOP"}
  }]
}`

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleReverseOK)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("latitude", "51.5"),
		conn("longitude", "-0.12"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["city"]).To(Equal("London"))
	Expect(out["postcode"]).To(Equal("SW1A 2AA"))
}

func TestExecuteInvalidLatitude(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleReverseOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("latitude", "north"),
		conn("longitude", "-0.12"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("latitude"))
}
