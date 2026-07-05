package ukgov_parliament_commons_divisions

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestCommonsDivisions(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		// bare array, PascalCase
		_, _ = w.Write([]byte(`[
		  {"DivisionId":1234,"Date":"2026-03-05T00:00:00","Number":120,"Title":"Budget Resolution","AyeCount":320,"NoCount":290},
		  {"DivisionId":1235,"Date":"2026-03-04T00:00:00","Number":119,"Title":"Amendment 3","AyeCount":180,"NoCount":400}
		]`))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "query", Type: core.ConnectionTypeString, Value: "budget"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	// The .json path segment must be present.
	Expect(gotPath).To(Equal("/data/divisions.json/search"))
	Expect(gotQuery).To(ContainSubstring("searchTerm=budget"))
	Expect(out["tool_result"]).To(ContainSubstring("Budget Resolution — Ayes 320, Noes 290 (2026-03-05)"))
}
