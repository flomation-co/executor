package http_request

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	. "github.com/onsi/gomega"
)

func strConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func textConn(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeText, Value: value}
}

func Test_GET_Request(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal("GET"))
		w.Header().Set("X-Test", "hello")
		w.WriteHeader(200)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("method", "GET"),
		strConn("url", server.URL),
	})
	Expect(err).To(BeNil())
	Expect(result["status_code"]).To(Equal(200))
	Expect(result["response_body"]).To(Equal(`{"status":"ok"}`))
	Expect(result["success"]).To(Equal(true))
}

func Test_POST_WithBody(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Expect(r.Method).To(Equal("POST"))
		Expect(r.Header.Get("Content-Type")).To(Equal("application/json"))
		w.WriteHeader(201)
		w.Write([]byte(`created`))
	}))
	defer server.Close()

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("method", "POST"),
		strConn("url", server.URL),
		textConn("headers", "Content-Type: application/json"),
		textConn("body", `{"name":"test"}`),
	})
	Expect(err).To(BeNil())
	Expect(result["status_code"]).To(Equal(201))
	Expect(result["success"]).To(Equal(true))
}

func Test_404_Response(t *testing.T) {
	RegisterTestingT(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte("not found"))
	}))
	defer server.Close()

	result, err := Execute(nil, nil, []*core.Connection{
		strConn("method", "GET"),
		strConn("url", server.URL),
	})
	Expect(err).To(BeNil())
	Expect(result["status_code"]).To(Equal(404))
	Expect(result["success"]).To(Equal(false))
}

func Test_MissingURL(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("method", "GET"),
	})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("URL is required"))
}

func Test_MissingMethod(t *testing.T) {
	RegisterTestingT(t)

	_, err := Execute(nil, nil, []*core.Connection{
		strConn("url", "https://example.com"),
	})
	Expect(err).ToNot(BeNil())
	Expect(err.Error()).To(ContainSubstring("method is required"))
}
