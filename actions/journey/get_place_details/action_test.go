package journey_get_place_details

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

const googlePlaceDetailsOK = `{
  "status": "OK",
  "result": {
    "place_id": "ChIJ1",
    "name": "Test Restaurant",
    "formatted_address": "1 Test Lane, London",
    "geometry": {"location": {"lat": 51.5, "lng": -0.1}},
    "rating": 4.7,
    "formatted_phone_number": "020 0000 0000",
    "opening_hours": {"weekday_text": ["Mon: 9-5"]}
  }
}`

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googlePlaceDetailsOK)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("place_id", "ChIJ1"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["name"]).To(Equal("Test Restaurant"))
	Expect(out["rating"]).To(Equal("4.7"))
	Expect(out["phone"]).To(Equal("020 0000 0000"))
	Expect(out["tool_result"]).To(ContainSubstring("Test Restaurant"))
	Expect(out["tool_result"]).To(ContainSubstring("4.7"))
}

func TestExecuteMissingPlaceID(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googlePlaceDetailsOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("place_id"))
}
