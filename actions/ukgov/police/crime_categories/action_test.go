package ukgov_police_crime_categories

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/police"
	. "github.com/onsi/gomega"
)

func TestCrimeCategories(t *testing.T) {
	RegisterTestingT(t)
	var gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.URL.Path).To(Equal("/crime-categories"))
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`[{"url":"all-crime","name":"All crime"},{"url":"burglary","name":"Burglary"}]`))
	}))
	defer srv.Close()

	old := police.BaseURL
	police.BaseURL = srv.URL
	defer func() { police.BaseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "date", Type: core.ConnectionTypeString, Value: "2024-01"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(gotQuery).To(ContainSubstring("date=2024-01"))
	Expect(out["tool_result"]).To(ContainSubstring("All crime"))
}
