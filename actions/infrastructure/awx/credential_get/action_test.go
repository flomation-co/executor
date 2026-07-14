package infrastructure_awx_credential_get

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

// ★ THE TRAP. AWX returns the LITERAL STRING "$encrypted$" for every secret input
// — it is a placeholder, not the secret, and not a redaction this node applied. A
// flow that wired result.inputs.password into an SSH step would authenticate with
// the word "$encrypted$". The summary has to say so, in words.
func TestGetCredentialSaysTheSecretsAreNotRealValues(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/api/v2/credentials/1/" {
			_, _ = w.Write([]byte(`{
				"id":1,"name":"Demo Credential","kind":"ssh","credential_type":1,"managed":false,
				"inputs":{"username":"deploy","password":"$encrypted$","ssh_key_data":"$encrypted$"},
				"summary_fields":{"credential_type":{"id":1,"name":"Machine"}}
			}`))
			return
		}
		t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL, str("credential_id", "1")))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["id"]).To(Equal("1"))
	Expect(out["name"]).To(Equal("Demo Credential"))
	Expect(out["kind"]).To(Equal("ssh"))
	Expect(out["credential_type"]).To(Equal("Machine")) // the human name, not "1"
	Expect(out["managed"]).To(Equal(false))

	// The encrypted field names are surfaced, sorted — and the non-secret one is not.
	Expect(out["encrypted_fields"]).To(Equal([]string{"password", "ssh_key_data"}))

	summary, _ := out["tool_result"].(string)
	Expect(summary).To(ContainSubstring("did not return the secret value of password, ssh_key_data"))
	Expect(summary).To(ContainSubstring("NOT the stored secret"))
}

func TestGetCredentialWithNoSecretsSaysSo(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"id":2,"name":"Ansible Galaxy","kind":"galaxy","managed":true,"inputs":{"url":"https://galaxy.ansible.com/"}}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL, str("credential_id", "2")))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(true))
	Expect(out["managed"]).To(Equal(true))
	Expect(out["encrypted_fields"]).To(Equal([]string{}))
	Expect(out["tool_result"]).To(ContainSubstring("managed by AWX"))
	Expect(out["tool_result"]).To(ContainSubstring("no secret fields"))
}

// AWX hides objects you cannot SEE behind a 404, so this must never be reported
// as "deleted" — and it must be SOFT, or one bad id kills the whole flow.
func TestGetCredentialNotFoundIsASoftFailure(t *testing.T) {
	RegisterTestingT(t)

	srv := awxServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"detail":"Not found."}`))
	})
	defer srv.Close()

	out, err := Execute(nil, nil, inputs(srv.URL, str("credential_id", "999")))

	Expect(err).To(BeNil())
	Expect(out["success"]).To(Equal(false))
	Expect(out["error"]).To(ContainSubstring("404"))
	Expect(out["error"]).To(ContainSubstring("permission to see it"))
}
