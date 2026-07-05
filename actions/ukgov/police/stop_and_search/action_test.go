package ukgov_police_stop_and_search

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/police"
	. "github.com/onsi/gomega"
)

// sample includes a boolean outcome (false) and a string outcome to exercise
// the interface{} typing of Stop.Outcome.
const sample = `[
  {"type":"Person search","datetime":"2024-01-05T12:00:00","object_of_search":"Controlled drugs","outcome":false,"gender":"Male","age_range":"18-24"},
  {"type":"Person search","datetime":"2024-01-06T13:00:00","object_of_search":"Controlled drugs","outcome":"A no further action disposal","gender":"Female","age_range":"25-34"},
  {"type":"Vehicle search","datetime":"2024-01-07T09:00:00","object_of_search":"Stolen goods","outcome":"Arrest","gender":"Male","age_range":"over 34"}
]`

func TestStopAndSearch(t *testing.T) {
	RegisterTestingT(t)
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	old := police.BaseURL
	police.BaseURL = srv.URL
	defer func() { police.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "latitude", Type: core.ConnectionTypeString, Value: "52.6"},
		{Name: "longitude", Type: core.ConnectionTypeString, Value: "-1.1"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(3))
	Expect(gotPath).To(Equal("/stops-street"))
	Expect(out["tool_result"]).To(ContainSubstring("Controlled drugs (2)"))
}
