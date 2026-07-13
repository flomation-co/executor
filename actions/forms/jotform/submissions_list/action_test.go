package submissions_list

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/forms/jotform"
	. "github.com/onsi/gomega"
)

func TestSubmissionsList(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal(http.MethodGet))
		Expect(r.URL.Path).To(Equal("/form/240000000001/submissions"))
		Expect(r.Header.Get("APIKEY")).To(Equal("key_123"))
		Expect(r.URL.Query().Get("limit")).To(Equal("2"))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"responseCode":200,"content":[{"id":"5551"},{"id":"5552"}]}`))
	}))
	defer srv.Close()

	old := jotform.BaseURL
	jotform.BaseURL = srv.URL
	defer func() { jotform.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "key_123"},
		{Name: "form_id", Type: core.ConnectionTypeString, Value: "240000000001"},
		{Name: "limit", Type: core.ConnectionTypeInteger, Value: "2"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["count"]).To(Equal(2))
	subs, ok := out["submissions"].([]map[string]interface{})
	Expect(ok).To(BeTrue())
	Expect(subs).To(HaveLen(2))
	Expect(out["tool_result"]).To(ContainSubstring("Retrieved 2 submission(s)"))
}

func TestSubmissionsListMissingFormID(t *testing.T) {
	RegisterTestingT(t)

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "key_123"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("form_id is required"))
}

func TestSubmissionsListAPIError(t *testing.T) {
	RegisterTestingT(t)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"responseCode":404,"message":"Form not found"}`))
	}))
	defer srv.Close()

	old := jotform.BaseURL
	jotform.BaseURL = srv.URL
	defer func() { jotform.BaseURL = old }()

	inputs := []*core.Connection{
		{Name: "api_key", Type: core.ConnectionTypeSecret, Value: "key_123"},
		{Name: "form_id", Type: core.ConnectionTypeString, Value: "bad"},
	}

	out, err := Execute(nil, nil, inputs)
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Form not found"))
}
