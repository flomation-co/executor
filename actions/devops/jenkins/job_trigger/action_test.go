package devops_jenkins_job_trigger

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func TestTriggerJob(t *testing.T) {
	RegisterTestingT(t)
	var path string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		Expect(r.Method).To(Equal(http.MethodPost))
		w.Header().Set("Location", "http://ci/queue/item/7/")
		w.WriteHeader(http.StatusCreated)
	}))
	defer srv.Close()

	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "base_url", Type: core.ConnectionTypeString, Value: srv.URL},
		{Name: "username", Type: core.ConnectionTypeString, Value: "admin"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "tok"},
		{Name: "job", Type: core.ConnectionTypeString, Value: "deploy"},
	})
	Expect(err).To(BeNil())
	Expect(path).To(Equal("/job/deploy/build"))
	Expect(out["success"]).To(Equal(true))
	Expect(out["queue_url"]).To(Equal("http://ci/queue/item/7/"))
}

func TestTriggerJobMissingJob(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, []*core.Connection{
		{Name: "base_url", Type: core.ConnectionTypeString, Value: "https://ci.example.com"},
		{Name: "username", Type: core.ConnectionTypeString, Value: "admin"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "tok"},
	})
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Job is required"))
}
