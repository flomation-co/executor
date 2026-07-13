package responses_list

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/forms/typeform"
	. "github.com/onsi/gomega"
)

func TestResponsesList(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodGet))
		Expect(r.URL.Path).To(Equal("/forms/form_abc/responses"))
		Expect(r.URL.Query().Get("page_size")).To(Equal("25"))
		Expect(r.URL.Query().Get("completed")).To(Equal("true"))
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer tok_123"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"total_items":2,"page_count":1,"items":[{"response_id":"r1"},{"response_id":"r2"}]}`))
	}))
	defer srv.Close()

	old := typeform.BaseURL
	typeform.BaseURL = srv.URL
	defer func() { typeform.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "tok_123"},
		{Name: "form_id", Type: core.ConnectionTypeString, Value: "form_abc"},
		{Name: "page_size", Type: core.ConnectionTypeInteger, Value: "25"},
		{Name: "completed", Type: core.ConnectionTypeString, Value: "true"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	Expect(out["total_items"]).To(Equal(2))
	Expect(out["tool_result"]).To(ContainSubstring("Retrieved 2 response(s)"))

	responses, ok := out["responses"].([]map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(responses).To(HaveLen(2))
}

func TestResponsesListMissingFormID(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "tok_123"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("form_id is required"))
}
