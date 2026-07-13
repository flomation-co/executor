package form_create

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/forms/googleforms"
	. "github.com/onsi/gomega"
)

func TestFormCreate(t *testing.T) {
	RegisterTestingT(t)

	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.URL.Path).To(Equal("/forms"))
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer tok_123"))
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"formId":"form_abc","responderUri":"https://docs.google.com/forms/d/e/form_abc/viewform","info":{"title":"Customer feedback"}}`))
	}))
	defer srv.Close()

	old := googleforms.BaseURL
	googleforms.BaseURL = srv.URL
	defer func() { googleforms.BaseURL = old }()

	// A non-${...} credential value is treated as a raw access token by
	// google_common.FetchTokens, which keeps the httptest hermetic.
	inputs := []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok_123"},
		{Name: "title", Type: core.ConnectionTypeString, Value: "Customer feedback"},
	}

	out, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["form_id"]).To(Equal("form_abc"))
	Expect(out["responder_uri"]).To(Equal("https://docs.google.com/forms/d/e/form_abc/viewform"))
	Expect(out["tool_result"]).To(ContainSubstring("Created Google Form"))

	// Verify the info.title/documentTitle body shape Google requires at create.
	var payload map[string]map[string]interface{}
	Expect(json.Unmarshal([]byte(gotBody), &payload)).To(Succeed())
	Expect(payload["info"]["title"]).To(Equal("Customer feedback"))
	Expect(payload["info"]["documentTitle"]).To(Equal("Customer feedback"))
}

func TestFormCreateMissingTitle(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "tok_123"},
	}

	out, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("title is required"))
}

func TestFormCreateAPIError(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403,"message":"The caller does not have permission"}}`))
	}))
	defer srv.Close()

	old := googleforms.BaseURL
	googleforms.BaseURL = srv.URL
	defer func() { googleforms.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "credential", Type: core.ConnectionTypeCredential, Value: "bad"},
		{Name: "title", Type: core.ConnectionTypeString, Value: "Customer feedback"},
	}

	out, err := Execute(&core.Flow{}, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("does not have permission"))
}
