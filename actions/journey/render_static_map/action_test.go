package journey_render_static_map

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
	body   []byte
}

func (s *stubTransport) RoundTrip(*http.Request) (*http.Response, error) {
	h := make(http.Header)
	h.Set("Content-Type", "image/png")
	return &http.Response{
		StatusCode: s.status,
		Body:       io.NopCloser(bytes.NewReader(s.body)),
		Header:     h,
	}, nil
}

func swapClient(body []byte) func() {
	prev := journey.DefaultClient
	journey.DefaultClient = &http.Client{Transport: &stubTransport{status: 200, body: body}, Timeout: time.Second}
	return func() { journey.DefaultClient = prev }
}

func conn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func TestExecuteSuccess(t *testing.T) {
	RegisterTestingT(t)
	pngBytes := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0x00, 0x00, 0x00, 0x0d}
	defer swapClient(pngBytes)()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("polyline", "_p~iF~ps|U"),
		conn("width", "800"),
		conn("height", "600"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["mime_type"]).To(Equal("image/png"))
	Expect(out["image_base64"]).ToNot(BeEmpty())
}

func TestExecuteMissingPolyline(t *testing.T) {
	RegisterTestingT(t)
	defer swapClient([]byte("png"))()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("polyline"))
}
