package responses_list

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/forms/surveymonkey"
	. "github.com/onsi/gomega"
)

func TestResponsesList(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodGet))
		Expect(r.URL.Path).To(Equal("/surveys/sv_abc/responses/bulk"))
		Expect(r.URL.Query().Get("page")).To(Equal("1"))
		Expect(r.URL.Query().Get("per_page")).To(Equal("50"))
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer tok_123"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total":2,"page":1,"data":[{"id":"r1"},{"id":"r2"}]}`))
	}))
	defer srv.Close()

	old := surveymonkey.BaseURL
	surveymonkey.BaseURL = srv.URL
	defer func() { surveymonkey.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok_123"},
		{Name: "survey_id", Type: core.ConnectionTypeString, Value: "sv_abc"},
		{Name: "page", Type: core.ConnectionTypeInteger, Value: "1"},
		{Name: "per_page", Type: core.ConnectionTypeInteger, Value: "50"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(out["total"]).To(Equal(2))
	Expect(out["tool_result"]).To(ContainSubstring("Retrieved 2 response(s)"))

	responses, ok := out["responses"].([]map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(responses).To(HaveLen(2))
}

func TestResponsesListMissingSurveyID(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok_123"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("survey_id is required"))
}
