package journey_geocode_address

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

type stubTransport struct {
	status int
	body   string
}

func (s *stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(bytes.NewBufferString(s.body)),
		Header:     make(http.Header),
	}, nil
}

func swapClient(body string) func() {
	prev := journey.DefaultClient
	journey.DefaultClient = &http.Client{Transport: &stubTransport{status: 200, body: body}, Timeout: time.Second}
	return func() { journey.DefaultClient = prev }
}

func conn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

const googleGeocodeOK = `{
  "status": "OK",
  "results": [{
    "formatted_address": "10 Downing St, London SW1A 2AA, UK",
    "geometry": {"location": {"lat": 51.5033635, "lng": -0.1276248}, "location_type": "ROOFTOP"}
  }]
}`

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleGeocodeOK)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("address", "10 Downing Street"),
		conn("region", "gb"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["formatted_address"]).To(Equal("10 Downing St, London SW1A 2AA, UK"))
	Expect(out["confidence"]).To(Equal("high"))
	Expect(out["tool_result"]).To(ContainSubstring("Downing St"))
}

func TestExecuteMissingAddress(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleGeocodeOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("address"))
}

func TestExecuteMissingAPIKey(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleGeocodeOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("address", "London"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("api_key"))
}

func TestExecuteProviderError(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(`{"status":"REQUEST_DENIED","error_message":"bad key"}`)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("address", "London"),
	})
	Expect(err).To(HaveOccurred())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("REQUEST_DENIED"))
}
