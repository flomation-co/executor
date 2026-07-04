package devops_jenkins_job_trigger_params

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestTriggerJobWithParameters(t *testing.T) {
	RegisterTestingT(t)
	var path, contentType, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		contentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "base_url", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "username", Type: core.ConnectionTypeString, Value: "admin"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "job", Type: core.ConnectionTypeString, Value: "deploy"},
		{Name: "parameters", Type: core.ConnectionTypeKeyValueArray,
			Value: `[{"key":"BRANCH","value":"main"},{"key":"ENV","value":"prod"}]`},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(path).To(Equal("/job/deploy/buildWithParameters"))
	Expect(contentType).To(Equal("application/x-www-form-urlencoded"))

	form, _ := url.ParseQuery(body)
	Expect(form.Get("BRANCH")).To(Equal("main"))
	Expect(form.Get("ENV")).To(Equal("prod"))
}
