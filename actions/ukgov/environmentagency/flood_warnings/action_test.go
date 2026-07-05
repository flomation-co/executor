package ukgov_environmentagency_flood_warnings

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/environmentagency"
	. "github.com/onsi/gomega"
)

const sample = `{"items":[
  {"description":"River Severn at Shrewsbury","severity":"Flood Warning","severityLevel":2,"floodAreaID":"111","floodArea":{"county":"Shropshire","riverOrSea":"River Severn"}},
  {"description":"Lower River Severn","severity":"Flood Alert","severityLevel":3,"floodAreaID":"112","floodArea":{"county":"Shropshire"}},
  {"description":"Upper River Severn","severity":"Flood Alert","severityLevel":3,"floodAreaID":"113","floodArea":{"county":"Shropshire"}}
]}`

func TestFloodWarnings(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sample))
	}))
	defer srv.Close()

	old := environmentagency.BaseURL
	environmentagency.BaseURL = srv.URL
	defer func() { environmentagency.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "county", Type: core.ConnectionTypeString, Value: "Shropshire"},
		{Name: "min_severity", Type: core.ConnectionTypeInteger, Value: int64(3)},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(3))
	Expect(gotPath).To(Equal("/id/floods"))
	Expect(gotQuery).To(ContainSubstring("county=Shropshire"))
	Expect(gotQuery).To(ContainSubstring("min-severity=3"))
	// Most severe = lowest severityLevel (2 = Flood Warning).
	Expect(out["tool_result"]).To(ContainSubstring("Most severe: River Severn at Shrewsbury"))
	Expect(out["tool_result"]).To(ContainSubstring("Flood Warning (1)"))
	Expect(out["tool_result"]).To(ContainSubstring("Flood Alert (2)"))
}

func TestFloodWarningsNone(t *testing.T) {
	RegisterTestingT(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[]}`))
	}))
	defer srv.Close()

	old := environmentagency.BaseURL
	environmentagency.BaseURL = srv.URL
	defer func() { environmentagency.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(0))
	Expect(out["tool_result"]).To(ContainSubstring("No active flood warnings"))
}
