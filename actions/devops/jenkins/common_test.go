package jenkins

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func strInput(name, val string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: val}
}

func authInputs(base string) []*core.Connection {
	return []*core.Connection{
		strInput("base_url", base),
		strInput("username", "admin"),
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "tok"},
	}
}

func TestGetConfigValidatesAndNormalises(t *testing.T) {
	RegisterTestingT(t)

	_, err := GetConfig([]*core.Connection{})
	Expect(err).To(HaveOccurred())

	cfg, err := GetConfig([]*core.Connection{
		strInput("base_url", "https://ci.example.com/"),
		strInput("username", "admin"),
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "tok"},
	})
	Expect(err).To(BeNil())
	Expect(cfg.BaseURL).To(Equal("https://ci.example.com")) // trailing slash trimmed
	Expect(cfg.Username).To(Equal("admin"))
	Expect(cfg.Token).To(Equal("tok"))

	// Missing token is rejected even though base_url/username are present.
	_, err = GetConfig([]*core.Connection{
		strInput("base_url", "https://ci.example.com"),
		strInput("username", "admin"),
	})
	Expect(err).To(HaveOccurred())
	Expect(err.Error()).To(ContainSubstring("API Token"))
}

func TestNormaliseBaseURL(t *testing.T) {
	RegisterTestingT(t)

	cases := map[string]string{
		"https://ci.example.com":          "https://ci.example.com",
		"https://ci.example.com/":         "https://ci.example.com",
		"http://192.168.0.5:8080/":        "http://192.168.0.5:8080",
		"https://host/jenkins/":           "https://host/jenkins",   // context path preserved
		"ci.example.com":                  "https://ci.example.com", // bare host → https
		"https://ci.example.com/?x=1#foo": "https://ci.example.com", // query/fragment stripped
	}
	for in, want := range cases {
		got, err := NormaliseBaseURL(in)
		Expect(err).To(BeNil(), in)
		Expect(got).To(Equal(want), in)
	}

	for _, bad := range []string{"", "   ", "ftp://x", "http://"} {
		_, err := NormaliseBaseURL(bad)
		Expect(err).To(HaveOccurred(), bad)
	}
}

func TestJobPath(t *testing.T) {
	RegisterTestingT(t)
	Expect(JobPath("build")).To(Equal("/job/build"))
	Expect(JobPath("/build/")).To(Equal("/job/build"))
	Expect(JobPath("folder/child")).To(Equal("/job/folder/job/child"))
	Expect(JobPath("has space")).To(Equal("/job/has%20space"))
	Expect(JobPath("")).To(Equal(""))
	Expect(JobPath("  ")).To(Equal(""))
}

func TestGetSendsBasicAuth(t *testing.T) {
	RegisterTestingT(t)
	var gotAuth, gotAccept string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotAccept = r.Header.Get("Accept")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	cfg, _ := GetConfig(authInputs(srv.URL))
	resp, err := Get(cfg, "/api/json")
	Expect(err).To(BeNil())
	Expect(resp.StatusCode).To(Equal(200))
	Expect(gotAuth).To(Equal("Basic " + base64.StdEncoding.EncodeToString([]byte("admin:tok"))))
	Expect(gotAccept).To(Equal("application/json"))
}

// TestPostRetriesWithCrumb reproduces the hardened / password-auth instance:
// the first POST is rejected 403 for a missing crumb, the client fetches one
// from /crumbIssuer/api/json, and the retried POST carries the crumb header.
func TestPostRetriesWithCrumb(t *testing.T) {
	RegisterTestingT(t)
	var posts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/crumbIssuer/api/json":
			_, _ = w.Write([]byte(`{"crumbRequestField":"Jenkins-Crumb","crumb":"abc123"}`))
		case r.URL.Path == "/job/x/build":
			n := atomic.AddInt32(&posts, 1)
			if n == 1 {
				Expect(r.Header.Get("Jenkins-Crumb")).To(BeEmpty())
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write([]byte("403 No valid crumb was included in the request"))
				return
			}
			Expect(r.Header.Get("Jenkins-Crumb")).To(Equal("abc123"))
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	cfg, _ := GetConfig(authInputs(srv.URL))
	resp, err := Post(cfg, "/job/x/build", "", nil)
	Expect(err).To(BeNil())
	Expect(resp.StatusCode).To(Equal(http.StatusCreated))
	Expect(atomic.LoadInt32(&posts)).To(Equal(int32(2)))
}

// A non-crumb 403 must surface as itself, not trigger an infinite retry.
func TestPostDoesNotRetryOnUnrelated403(t *testing.T) {
	RegisterTestingT(t)
	var posts int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&posts, 1)
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("access denied"))
	}))
	defer srv.Close()

	cfg, _ := GetConfig(authInputs(srv.URL))
	resp, err := Post(cfg, "/job/x/build", "", nil)
	Expect(err).To(BeNil())
	Expect(resp.StatusCode).To(Equal(http.StatusForbidden))
	Expect(atomic.LoadInt32(&posts)).To(Equal(int32(1)))
}

func TestCheckResponse(t *testing.T) {
	RegisterTestingT(t)
	Expect(CheckResponse(&Response{StatusCode: 200})).To(BeNil())
	Expect(CheckResponse(&Response{StatusCode: 201}, 201)).To(BeNil())
	Expect(CheckResponse(&Response{StatusCode: 503}, 503)).To(BeNil())

	err := CheckResponse(&Response{StatusCode: 401})
	Expect(err.Error()).To(ContainSubstring("unauthorised"))

	err = CheckResponse(&Response{StatusCode: 404})
	Expect(err.Error()).To(ContainSubstring("404"))

	err = CheckResponse(&Response{StatusCode: 500, Body: []byte("<html><body><h1>Oops</h1> something broke</body></html>")})
	Expect(err.Error()).To(ContainSubstring("500"))
	Expect(err.Error()).To(ContainSubstring("Oops"))
	Expect(err.Error()).NotTo(ContainSubstring("<h1>"))
}

func TestKeyValuesFromArray(t *testing.T) {
	RegisterTestingT(t)
	conn := &core.Connection{
		Name:  "parameters",
		Type:  core.ConnectionTypeKeyValueArray,
		Value: `[{"key":"BRANCH","value":"main"},{"key":"","value":"skipme"},{"key":"DEPLOY","value":"true"}]`,
	}
	v := KeyValues("parameters", []*core.Connection{conn})
	Expect(v.Get("BRANCH")).To(Equal("main"))
	Expect(v.Get("DEPLOY")).To(Equal("true"))
	Expect(v.Encode()).NotTo(ContainSubstring("skipme")) // blank key skipped
}

func TestResultShaping(t *testing.T) {
	RegisterTestingT(t)
	r := ResourceResult(map[string]interface{}{"name": "x"}, "ok")
	Expect(r["success"]).To(Equal(true))
	Expect(r["result"]).To(HaveKeyWithValue("name", "x"))

	l := ListResult(nil, "empty")
	Expect(l["count"]).To(Equal(0))
	Expect(l["results"]).To(Equal([]interface{}{}))

	s := SuccessResult("done", map[string]interface{}{"queue_url": "u"})
	Expect(s["queue_url"]).To(Equal("u"))
	Expect(s["success"]).To(Equal(true))

	e := ErrorResult("boom")
	Expect(e["success"]).To(Equal(false))
	Expect(e["error"]).To(Equal("boom"))
}

func TestOptionalStringGuardsNilCodeSecret(t *testing.T) {
	RegisterTestingT(t)
	// A Code/Secret input with a nil value must read as "" (not the literal
	// "<nil>" that fmt.Sprintf would otherwise produce for those types).
	inputs := []*core.Connection{
		{Name: "xml", Type: core.ConnectionTypeCode, Value: nil},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: nil},
	}
	Expect(OptionalString("xml", inputs)).To(Equal(""))
	Expect(OptionalString("api_token", inputs)).To(Equal(""))
	Expect(strings.TrimSpace(OptionalString("missing", inputs))).To(Equal(""))
}
