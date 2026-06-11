package journey_find_arrive_by

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

func distanceMatrixBody(durationSecs, distanceMetres int) string {
	return `{"status":"OK","rows":[{"elements":[{"status":"OK","distance":{"value":` +
		strconv.Itoa(distanceMetres) + `},"duration":{"value":` + strconv.Itoa(durationSecs) + `}}]}]}`
}

func TestExecuteTwoPassEstimation(t *testing.T) {
	RegisterTestingT(t)
	// First call (no depart_at): free-flow 30 min. Second call (with
	// candidate depart_at): traffic-aware 45 min. Recommended departure
	// should be arrive_by - 45min.
	cleanup, tr := swapClient([]string{
		distanceMatrixBody(1800, 30000),
		distanceMatrixBody(2700, 30000),
	})
	defer cleanup()

	out, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("origin", "Home"),
		conn("destination", "Office"),
		conn("arrive_by", "2026-06-12T09:00:00Z"),
	})
	Expect(err).ToNot(HaveOccurred())
	Expect(out["success"]).To(Equal(true))
	Expect(out["estimated_duration_friendly"]).To(Equal("45 minutes"))
	Expect(out["recommended_departure"]).To(Equal("2026-06-12T08:15:00Z"))
	Expect(tr.calls).To(Equal(2))
}

func TestExecuteRequiresArriveBy(t *testing.T) {
	RegisterTestingT(t)
	cleanup, _ := swapClient([]string{distanceMatrixBody(1800, 30000)})
	defer cleanup()

	_, err := Execute(nil, nil, []*core.Connection{
		conn("provider", "google"),
		conn("api_key", "test-key"),
		conn("origin", "Home"),
		conn("destination", "Office"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("arrive_by"))
}
