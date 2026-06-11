package journey_generate_itinerary_pdf

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
	body []byte
}

func (s *stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	h := make(http.Header)
	h.Set("Content-Type", "image/png")
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(bytes.NewReader(s.body)),
		Header:     h,
	}, nil
}

func swapClient(body []byte) func() {
	prev := journey.DefaultClient
	journey.DefaultClient = &http.Client{Transport: &stubTransport{body: body}, Timeout: time.Second}
	return func() { journey.DefaultClient = prev }
}

func conn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func boolConn(name string, value bool) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeBoolean, Value: value}
}

func textConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: value}
}

// PDF tests bypass the static map fetch by setting include_map=false. This
// keeps tests fast and independent of fpdf's image decoder — the image
// embedding code path is covered indirectly via the static_map provider
// tests in journey_common.

func TestExecuteSuccessNoMap(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(nil)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("polyline", "_p~iF~ps|U"),
		conn("title", "London to Manchester"),
		conn("distance_miles", "200.0"),
		conn("duration_friendly", "3 hours 45 minutes"),
		textConn("steps_json", `[{"instruction":"Head north on A40","distance_metres":500,"duration_seconds":60}]`),
		boolConn("include_map", false),
		boolConn("include_directions", true),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["page_count"]).To(Equal("1"))
	Expect(out["pdf_base64"]).ToNot(BeEmpty())
}

func TestExecuteMissingPolyline(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(nil)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("polyline"))
}

func TestExecuteInvalidStepsJSONFailsLoudly(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(nil)()

	// Malformed steps_json must surface as a node failure so flow authors
	// notice the wiring problem — silently dropping directions used to hide
	// the upstream substitution bug (Go syntax leaking through instead of JSON).
	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("polyline", "_p~iF~ps|U"),
		textConn("steps_json", "{not json"),
		boolConn("include_map", false),
		boolConn("include_directions", true),
	})
	Expect(err).To(HaveOccurred())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("steps_json"))
}
