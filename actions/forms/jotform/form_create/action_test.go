package form_create

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/forms/jotform"
	. "github.com/onsi/gomega"
)

func TestFormCreate(t *testing.T) {
	RegisterTestingT(t)

	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.URL.Path).To(Equal("/user/forms"))
		Expect(r.Header.Get("APIKEY")).To(Equal("key_123"))
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"responseCode":200,"content":{"id":"240000000001","title":"My form"}}`))
	}))
	defer srv.Close()

	old := jotform.BaseURL
	jotform.BaseURL = srv.URL
	defer func() { jotform.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "key_123"},
		{Name: "region", Type: core.ConnectionTypeString, Value: "eu"},
		{Name: "questions", Type: core.ConnectionTypeString, Value: `{"1":{"type":"control_textbox","text":"Name"}}`},
		{Name: "properties", Type: core.ConnectionTypeString, Value: `{"title":"My form"}`},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["form_id"]).To(Equal("240000000001"))
	Expect(out["tool_result"]).To(ContainSubstring("Created JotForm form"))
	Expect(gotBody).To(ContainSubstring(`"questions"`))
	Expect(gotBody).To(ContainSubstring(`"properties"`))
}

func TestFormCreateInvalidQuestionsJSON(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "key_123"},
		{Name: "questions", Type: core.ConnectionTypeString, Value: "not json"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Invalid questions JSON"))
}

func TestFormCreateAPIError(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"responseCode":401,"message":"Invalid API key"}`))
	}))
	defer srv.Close()

	old := jotform.BaseURL
	jotform.BaseURL = srv.URL
	defer func() { jotform.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "bad"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Invalid API key"))
}
