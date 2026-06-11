package journey_get_travel_time

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

const googleDistanceMatrixOK = `{"status":"OK","rows":[{"elements":[{"status":"OK","distance":{"value":200000},"duration":{"value":7200}}]}]}`

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleDistanceMatrixOK)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("origin", "London"),
		conn("destination", "Manchester"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["duration_friendly"]).To(Equal("2 hours"))
	Expect(out["tool_result"]).To(ContainSubstring("2 hours"))
}

func TestExecuteMissingOrigin(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient(googleDistanceMatrixOK)()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("destination", "Manchester"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("origin"))
}
