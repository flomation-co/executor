package ukgov_parliament_written_questions

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestWrittenQuestions(t *testing.T) {
	RegisterTestingT(t)
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"totalResults":1,"results":[
		  {"value":{"id":99,"uin":"7245","house":"Commons","dateTabled":"2026-02-01T00:00:00","questionText":"To ask the Secretary of State for Health what steps are being taken on NHS waiting times.","answeringBodyName":"Department of Health and Social Care","answerText":"...","dateAnswered":"2026-02-05T00:00:00"}}
		]}`))
	}))
	defer srv.Close()

	old := baseURL
	baseURL = srv.URL
	defer func() { baseURL = old }()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "query", Type: core.ConnectionTypeString, Value: "NHS waiting times"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(1))
	Expect(out["total"]).To(Equal(1))
	Expect(gotPath).To(Equal("/api/writtenquestions/questions"))
	Expect(gotQuery).To(ContainSubstring("searchTerm=NHS+waiting+times"))
	Expect(out["tool_result"]).To(ContainSubstring("Department of Health and Social Care"))
	Expect(out["tool_result"]).To(ContainSubstring("tabled 2026-02-01"))
}

func TestTruncate(t *testing.T) {
	RegisterTestingT(t)
	Expect(truncate("short", 100)).To(Equal("short"))
	Expect(truncate("abcdefghij", 5)).To(Equal("abcde…"))
}
