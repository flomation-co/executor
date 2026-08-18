package facebook_get_pages

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core "flomation.app/automate/executor"
	fb "flomation.app/automate/executor/actions/social/facebook"
	. "github.com/onsi/gomega"
)

// tool_result is what an AI caller reads. The outputs below it are available to
// a wired flow but invisible to an agent deciding what to do next — so a summary
// naming only the page meant an agent could confirm the page existed and still
// not know its ID. That is a short step from inventing one and sending it to a
// live API.
func TestExecute_SummaryCarriesPageIDs(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]interface{}{
			{"id": "839839769201980", "name": "Flomation", "access_token": "PAGETOKEN", "category": "Software"},
			{"id": "111122223333444", "name": "Second Page", "access_token": "T2", "category": "Brand"},
		}})
	}))
	defer srv.Close()
	old := fb.GraphAPIBase
	fb.GraphAPIBase = srv.URL
	defer func() { fb.GraphAPIBase = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "EAAG"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))

	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring("839839769201980"), "the ID must be readable, not only in the outputs")
	Expect(summary).To(ContainSubstring("Flomation"))
	// Every page, not just the first — first_page_id already covered that case.
	Expect(summary).To(ContainSubstring("111122223333444"))
	Expect(strings.Count(summary, "ID:")).To(Equal(2))

	Expect(out["first_page_id"]).To(Equal("839839769201980"))
	Expect(out["page_count"]).To(Equal(int64(2)))
}

func TestExecute_NoPages(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer srv.Close()
	old := fb.GraphAPIBase
	fb.GraphAPIBase = srv.URL
	defer func() { fb.GraphAPIBase = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "EAAG"},
	})
	Expect(err).To(BeNil())
	Expect(out["tool_result"]).To(ContainSubstring("Found 0 page(s)"))
}
