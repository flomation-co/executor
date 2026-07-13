package form_create

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/forms/typeform"
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
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"form_abc","title":"My form","_links":{"display":"https://form.typeform.com/to/form_abc"}}`))
	}))
	defer srv.Close()

	old := typeform.BaseURL
	typeform.BaseURL = srv.URL
	defer func() { typeform.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "tok_123"},
		{Name: "title", Type: core.ConnectionTypeString, Value: "My form"},
		{Name: "fields", Type: core.ConnectionTypeString, Value: `[{"title":"Name","type":"short_text"}]`},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["form_id"]).To(Equal("form_abc"))
	Expect(out["form_url"]).To(Equal("https://form.typeform.com/to/form_abc"))
	Expect(out["tool_result"]).To(ContainSubstring("Created Typeform form"))
	Expect(gotBody).To(ContainSubstring(`"fields"`))
}

func TestFormCreateInvalidFieldsJSON(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "tok_123"},
		{Name: "title", Type: core.ConnectionTypeString, Value: "My form"},
		{Name: "fields", Type: core.ConnectionTypeString, Value: "not json"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Invalid fields JSON"))
}

func TestFormCreateAPIError(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":"UNAUTHORIZED","description":"Authentication credentials not found."}`))
	}))
	defer srv.Close()

	old := typeform.BaseURL
	typeform.BaseURL = srv.URL
	defer func() { typeform.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "bad"},
		{Name: "title", Type: core.ConnectionTypeString, Value: "My form"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Authentication credentials not found"))
}
