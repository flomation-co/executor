package ukgov_companieshouse_list_psc

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/ukgov/companieshouse"
	. "github.com/onsi/gomega"
)

func mockCH(status int, body string) func() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	old := companieshouse.BaseURL
	companieshouse.BaseURL = srv.URL
	return func() {
		companieshouse.BaseURL = old
		srv.Close()
	}
}

func inputs() []*core.Connection {
	return []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "testkey"},
		{Name: "company_number", Type: core.ConnectionTypeString, Value: "12345678"},
	}
}

func TestListPSC(t *testing.T) {
	RegisterTestingT(t)
	restore := mockCH(http.StatusOK, `{"total_results":1,"active_count":1,"items":[{"name":"John Smith","natures_of_control":["ownership-of-shares-75-to-100-percent"],"nationality":"British"}]}`)
	defer restore()

	out, err := Execute(nil, nil, inputs())
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(out["tool_result"]).To(ContainSubstring("John Smith"))
	Expect(out["tool_result"]).To(ContainSubstring("ownership-of-shares-75-to-100-percent"))
}

func TestListPSCNone(t *testing.T) {
	RegisterTestingT(t)
	// 404 means "no PSC register" — treated as a valid zero result.
	restore := mockCH(http.StatusNotFound, "")
	defer restore()

	out, err := Execute(nil, nil, inputs())
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(0))
	Expect(out["tool_result"]).To(ContainSubstring("no persons with significant control"))
}
