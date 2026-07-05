package ukgov_environmentagency_flood_areas

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/environmentagency"
	. "github.com/onsi/gomega"
)

func TestFloodAreas(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"items":[{"description":"River Severn area","county":"Shropshire","label":"River Severn at Shrewsbury","notation":"111","lat":52.7,"long":-2.75}]}`))
	}))
	defer srv.Close()

	old := environmentagency.BaseURL
	environmentagency.BaseURL = srv.URL
	defer func() { environmentagency.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "latitude", Type: core.ConnectionTypeString, Value: "52.7"},
		{Name: "longitude", Type: core.ConnectionTypeString, Value: "-2.75"},
		{Name: "distance_km", Type: core.ConnectionTypeInteger, Value: int64(15)},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(gotPath).To(Equal("/id/floodAreas"))
	Expect(gotQuery).To(ContainSubstring("long=-2.75"))
	Expect(gotQuery).To(ContainSubstring("dist=15"))
	Expect(out["tool_result"]).To(ContainSubstring("River Severn at Shrewsbury"))
}

func TestFloodAreasRequiresCoords(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "latitude", Type: core.ConnectionTypeString, Value: "52.7"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("longitude is required"))
}
