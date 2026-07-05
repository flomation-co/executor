package ukgov_companieshouse_get_insolvency

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

func TestGetInsolvency(t *testing.T) {
	RegisterTestingT(t)
	restore := mockCH(http.StatusOK, `{"cases":[{"number":"1","type":"creditors-voluntary-liquidation","dates":[{"date":"2022-01-01","type":"wound-up-on"}],"practitioners":[{"name":"A Liquidator","role":"practitioner"}]}]}`)
	defer restore()

	out, err := Execute(nil, nil, inputs())
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(out["tool_result"]).To(ContainSubstring("creditors-voluntary-liquidation"))
}

func TestGetInsolvencyNone(t *testing.T) {
	RegisterTestingT(t)
	restore := mockCH(http.StatusNotFound, "")
	defer restore()

	out, err := Execute(nil, nil, inputs())
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(0))
	Expect(out["tool_result"]).To(ContainSubstring("no insolvency history"))
}
