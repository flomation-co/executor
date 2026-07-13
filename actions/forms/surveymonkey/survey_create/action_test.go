package survey_create

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/forms/surveymonkey"
	. "github.com/onsi/gomega"
)

func TestSurveyCreate(t *testing.T) {
	RegisterTestingT(t)

	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodPost))
		Expect(r.URL.Path).To(Equal("/surveys"))
		Expect(r.Header.Get("Authorization")).To(Equal("Bearer tok_123"))
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"id":"sv_abc","title":"My survey","href":"https://api.surveymonkey.com/v3/surveys/sv_abc"}`))
	}))
	defer srv.Close()

	old := surveymonkey.BaseURL
	surveymonkey.BaseURL = srv.URL
	defer func() { surveymonkey.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok_123"},
		{Name: "title", Type: core.ConnectionTypeString, Value: "My survey"},
		{Name: "body", Type: core.ConnectionTypeString, Value: `{"nickname":"CSAT"}`},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["survey_id"]).To(Equal("sv_abc"))
	Expect(out["survey_url"]).To(Equal("https://api.surveymonkey.com/v3/surveys/sv_abc"))
	Expect(out["tool_result"]).To(ContainSubstring("Created SurveyMonkey survey"))
	Expect(gotBody).To(ContainSubstring(`"title":"My survey"`))
	Expect(gotBody).To(ContainSubstring(`"nickname":"CSAT"`))
}

func TestSurveyCreateInvalidBodyJSON(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok_123"},
		{Name: "title", Type: core.ConnectionTypeString, Value: "My survey"},
		{Name: "body", Type: core.ConnectionTypeString, Value: "not json"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Invalid body JSON"))
}

func TestSurveyCreateMissingTitle(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "tok_123"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("title is required"))
}

func TestSurveyCreateAPIError(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"id":"1010","name":"Unauthorized","message":"Authorization has been denied for this request."}}`))
	}))
	defer srv.Close()

	old := surveymonkey.BaseURL
	surveymonkey.BaseURL = srv.URL
	defer func() { surveymonkey.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "access_token", Type: core.ConnectionTypeSecret, Value: "bad"},
		{Name: "title", Type: core.ConnectionTypeString, Value: "My survey"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Authorization has been denied"))
}
