package environment

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	. "github.com/onsi/gomega"
)

// newTestEnvironment stands up an httptest server that serves the environment
// summary (needed by the constructor) and a credential endpoint returning the
// given JSON body, then builds an Environment pointed at it.
func newTestEnvironment(t *testing.T, credentialJSON string) *Environment {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "/credential/"):
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(credentialJSON))
		default:
			// environment summary fetched by NewEnvironment
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"id":"env-123"}`))
		}
	}))
	t.Cleanup(srv.Close)

	url := srv.URL
	env, err := NewEnvironment("default", &url, "exec-1", nil)
	Expect(err).To(BeNil())
	return env
}

func TestGetCredentialReturnsToken(t *testing.T) {
	RegisterTestingT(t)
	env := newTestEnvironment(t, `{"name":"MyXero","value":"access-tok-123","metadata":{"tenant_id":"ten-abc"}}`)

	tok, err := env.GetCredential("MyXero")
	Expect(err).To(BeNil())
	Expect(tok).To(Not(BeNil()))
	Expect(*tok).To(Equal("access-tok-123"))
}

func TestGetCredentialMetaResolvesKey(t *testing.T) {
	RegisterTestingT(t)
	env := newTestEnvironment(t, `{"name":"MyXero","value":"tok","metadata":{"tenant_id":"ten-abc","tenant_name":"Demo Co"}}`)

	v, err := env.GetCredentialMeta("MyXero", "tenant_id")
	Expect(err).To(BeNil())
	Expect(v).To(Not(BeNil()))
	Expect(*v).To(Equal("ten-abc"))

	// A key that isn't present resolves to (nil, nil).
	missing, err := env.GetCredentialMeta("MyXero", "realm_id")
	Expect(err).To(BeNil())
	Expect(missing).To(BeNil())
}

func TestGetCredentialMetaStringifiesNonString(t *testing.T) {
	RegisterTestingT(t)
	// QuickBooks realmId can arrive as a JSON number; it must stringify.
	env := newTestEnvironment(t, `{"name":"MyQBO","value":"tok","metadata":{"realm_id":9130347}}`)

	v, err := env.GetCredentialMeta("MyQBO", "realm_id")
	Expect(err).To(BeNil())
	Expect(v).To(Not(BeNil()))
	Expect(*v).To(Equal("9130347"))
}

func TestGetCredentialMetaNoMetadata(t *testing.T) {
	RegisterTestingT(t)
	env := newTestEnvironment(t, `{"name":"Plain","value":"tok"}`)

	v, err := env.GetCredentialMeta("Plain", "tenant_id")
	Expect(err).To(BeNil())
	Expect(v).To(BeNil())
}
