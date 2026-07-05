package ukgov_bankholidays_list

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

const sample = `{
  "england-and-wales": {"division":"england-and-wales","events":[
    {"title":"New Year's Day","date":"2026-01-01","notes":"","bunting":true},
    {"title":"Good Friday","date":"2026-04-03","notes":"","bunting":false},
    {"title":"Christmas Day","date":"2026-12-25","notes":"","bunting":true}
  ]},
  "scotland": {"division":"scotland","events":[
    {"title":"St Andrew's Day","date":"2026-11-30","notes":"","bunting":true}
  ]}
}`

func mock() func() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(sample))
	}))
	old := baseURL
	baseURL = srv.URL
	return func() { baseURL = old; srv.Close() }
}

func TestListBankHolidaysDefaultRegion(t *testing.T) {
	RegisterTestingT(t)
	restore := mock()
	defer restore()

	out, err := Execute(nil, nil, []*core.Connection{})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(3)) // england-and-wales default
	Expect(out["tool_result"]).To(ContainSubstring("england-and-wales"))
}

func TestListBankHolidaysScotlandAndYear(t *testing.T) {
	RegisterTestingT(t)
	restore := mock()
	defer restore()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "division", Type: core.ConnectionTypeString, Value: "scotland"},
		{Name: "year", Type: core.ConnectionTypeString, Value: "2026"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(out["tool_result"]).To(ContainSubstring("St Andrew's Day"))
}

func TestListBankHolidaysInvalidRegion(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "division", Type: core.ConnectionTypeString, Value: "wales-only"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Unknown region"))
}

func TestNextUpcoming(t *testing.T) {
	RegisterTestingT(t)
	events := []event{
		{Title: "Past", Date: "2026-01-01"},
		{Title: "Future", Date: "2026-12-25"},
	}
	title, date := nextUpcoming(events, time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	Expect(title).To(Equal("Future"))
	Expect(date).To(Equal("2026-12-25"))
}
