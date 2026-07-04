package devops_jenkins_job_create

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func auth(url string, extra ...*core.Connection) []*core.Connection {
	base := []*core.Connection{
		{Name: "base_url", Type: core.ConnectionTypeString, Value: url},
		{Name: "username", Type: core.ConnectionTypeString, Value: "admin"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "tok"},
	}
	return append(base, extra...)
}

func TestCreateJobPostsXML(t *testing.T) {
	RegisterTestingT(t)
	var path, query, contentType, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		query = r.URL.RawQuery
		contentType = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	out, err := Execute(nil, nil, auth(srv.URL,
		&core.Connection{Name: "new_job", Type: core.ConnectionTypeString, Value: "my job"},
		&core.Connection{Name: "xml", Type: core.ConnectionTypeCode, Value: "<project></project>"},
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(path).To(Equal("/createItem"))
	Expect(query).To(Equal("name=my+job")) // name goes in the query, url-encoded
	Expect(contentType).To(Equal("application/xml"))
	Expect(body).To(Equal("<project></project>"))
}

func TestCreateJobMissingXML(t *testing.T) {
	RegisterTestingT(t)
	out, err := Execute(nil, nil, auth("https://ci.example.com",
		&core.Connection{Name: "new_job", Type: core.ConnectionTypeString, Value: "j"},
	))
	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Config XML is required"))
}
