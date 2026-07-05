package ukgov_parliament_search_bills

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestSearchBills(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"totalResults":2,"items":[
		  {"billId":3773,"shortTitle":"Finance Act 2025","isAct":true,"currentHouse":"Commons","currentStage":{"description":"Royal Assent","house":"Commons"}},
		  {"billId":3800,"shortTitle":"Finance (No. 2) Bill","isAct":false,"currentStage":{"description":"2nd reading","house":"Commons"}}
		]}`))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "query", Type: core.ConnectionTypeString, Value: "finance"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(out["total"]).To(Equal(2))
	Expect(gotPath).To(Equal("/api/v1/Bills"))
	Expect(gotQuery).To(ContainSubstring("SearchTerm=finance"))
	Expect(out["tool_result"]).To(ContainSubstring("Finance Act 2025 (Royal Assent)"))
}
