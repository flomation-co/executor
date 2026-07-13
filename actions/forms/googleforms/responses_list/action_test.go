package responses_list

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/forms/googleforms"
	. "github.com/onsi/gomega"
)

func TestResponsesList(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodGet))
		Expect(r.URL.Path).To(Equal("/forms/form_abc/responses"))
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer tok_123"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"responses":[{"responseId":"r1"},{"responseId":"r2"}]}`))
	}))
	defer srv.Close()

	old := googleforms.BaseURL
	googleforms.BaseURL = srv.URL
	defer func() { googleforms.BaseURL = old }()

	// A non-${...} credential value is treated as a raw access token by
	// google_common.FetchTokens, keeping the httptest hermetic.
	inputs := []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok_123"},
		{Name: "form_id", Type: core.ConnectionTypeString, Value: "form_abc"},
	}

	out, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["total"]).To(Equal(2))
	Expect(out["tool_result"]).To(ContainSubstring("Retrieved 2 response(s)"))

	responses, ok := out["responses"].([]map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(responses).To(HaveLen(2))
}

func TestResponsesListMissingFormID(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok_123"},
	}

	out, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("form_id is required"))
}

func TestResponsesListAPIError(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"error":{"code":404,"message":"Requested entity was not found."}}`))
	}))
	defer srv.Close()

	old := googleforms.BaseURL
	googleforms.BaseURL = srv.URL
	defer func() { googleforms.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok_123"},
		{Name: "form_id", Type: core.ConnectionTypeString, Value: "missing"},
	}

	out, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("was not found"))
}
