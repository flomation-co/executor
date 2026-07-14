package infrastructure_awx_credential_delete

import (
	"net/http"
	"net/http/httptest"
	"testing"

	core "flomation.app/automate/executor"
	"flomation.app/automate/executor/actions/infrastructure/awx"
	. "github.com/onsi/gomega"
)

func awxServer(h http.HandlerFunc) *httptest.Server {
	awx.ResetAPIRootCacheForTest()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/":
			_, _ = w.Write([]byte(`{"current_version":"/api/v2/","available_versions":{"v2":"/api/v2/"}}`))
		case "/api/v2/me/":
			_, _ = w.Write([]byte(`{"count":1,"results":[{"id":1,"username":"admin"}]}`))
		default:
			h(w, r)
		}
	}))
}

func inputs(url string, extra ...*core.Connection) []*core.Connection {
	base := []*core.Connection{
		{Name: "awx_url", Type: core.ConnectionTypeString, Value: url},
		{Name: "auth_method", Type: core.ConnectionTypeString, Value: "token"},
		{Name: "api_token", Type: core.ConnectionTypeSecret, Value: "DIwkbhgP6oT0AAeOY9kUr7po1QBkYr"},
	}
	return append(base, extra...)
}

func str(name, value string) *core.Connection {
	return &core.Connection{Name: name, Type: core.ConnectionTypeString, Value: value}
}

func confirmed() *core.Connection {
	return &core.Connection{Name: "confirm_destructive", Type: core.ConnectionTypeBoolean, Value: true}
}

func TestDeleteCredentialSendsTheDelete(t *testing.T) {
	RegisterTestingT(t)

	var method, path string
	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		method, path = r.Method, r.URL.Path
		w.WriteHeader(http.StatusNoContent)
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL, str("credential_id", "9"), confirmed()))

	Expect(err).To(BeNil())
	Expect(method).To(Equal(http.MethodDelete))
	Expect(path).To(Equal("/api/v2/credentials/9/")) // ★ the trailing slash is mandatory
	Expect(out["success"]).To(Equal(true))
	Expect(out["deleted"]).To(Equal(true))
	Expect(out["id"]).To(Equal("9"))
}

// The guard fails CLOSED: nothing is sent to AWX at all.
func TestDeleteCredentialRefusesWithoutConfirmation(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("called AWX without confirmation: %s %s", r.Method, r.URL.Path)
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL, str("credential_id", "9")))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("Confirm Destructive Action"))
}

// ★ THE TRAP. AWX-managed credentials answer 403 "Deletion not allowed for managed
// credentials". The generic 403 ("this user does not have permission") would send
// the operator hunting for a role that does not exist.
func TestDeleteCredentialRewordsTheManagedCredential403(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"detail":"Deletion not allowed for managed credentials"}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL, str("credential_id", "2"), confirmed()))

	Expect(err).To(BeNil()) // SOFT
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("managed by AWX itself and can never be deleted"))
	Expect(out["error"]).ToNot(ContainSubstring("does not have permission"))
}
