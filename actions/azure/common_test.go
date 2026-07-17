package azure

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func resetCreds() {
	credCache.mu.Lock()
	credCache.m = map[string]*azidentity.ClientSecretCredential{}
	credCache.mu.Unlock()
}

// pinAuthority points the exchange at a test server for one test. This is the
// same knob a sovereign cloud would use (cloud.Configuration's
// ActiveDirectoryAuthorityHost), so the seam is real behaviour rather than a
// test-only backdoor.
func pinAuthority(t *testing.T, srv *httptest.Server, trustCert bool) {
	t.Helper()
	prevHost, prevDisc, prevTr := authorityHost, disableInstanceDiscovery, credTransport
	authorityHost = srv.URL
	// azidentity refuses a plain-HTTP authority outright, so these servers are
	// all TLS. Trusting the test cert is what lets the success path run at all;
	// the MITM test deliberately does NOT trust it.
	if trustCert {
		credTransport = srv.Client()
	}
	// A test server is not a known Microsoft instance, so MSAL's authority
	// validation would reject it before any exchange happened. Production
	// leaves discovery ON — see disableInstanceDiscovery.
	disableInstanceDiscovery = true
	resetCreds()
	t.Cleanup(func() {
		authorityHost, disableInstanceDiscovery, credTransport = prevHost, prevDisc, prevTr
		resetCreds()
	})
}

// tokenEndpoint stands in for login.microsoftonline.com. MSAL does not assume
// the token URL: it fetches the tenant's OpenID configuration first and uses
// the token_endpoint from it, so a usable fake has to serve that document too.
// tokenHits counts only the actual exchanges, so caching can be asserted.
func tokenEndpoint(t *testing.T, tokenHits *int) *httptest.Server {
	t.Helper()
	var mu sync.Mutex
	srv := httptest.NewUnstartedServer(nil)
	base := "https://" + srv.Listener.Addr().String()
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration"):
			_, _ = w.Write([]byte(`{
				"issuer": "` + base + `/tenant/v2.0",
				"authorization_endpoint": "` + base + `/tenant/oauth2/v2.0/authorize",
				"token_endpoint": "` + base + `/tenant/oauth2/v2.0/token"
			}`))
		case strings.HasSuffix(r.URL.Path, "/token"):
			mu.Lock()
			if tokenHits != nil {
				*tokenHits++
			}
			mu.Unlock()
			_, _ = w.Write([]byte(`{"token_type":"Bearer","expires_in":3599,"ext_expires_in":3599,"access_token":"TOK"}`))
		default:
			t.Logf("unexpected MSAL call: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	})
	srv.StartTLS()
	return srv
}

// TestTokenExchangeRefusesForgedCertificate is the regression guard for a real
// defect. The exchange used to take an *http.Client from its callers, and both
// the storage and cosmosdb packages handed it the client built from their
// allow_insecure checkbox — so ticking "allow insecure TLS" to reach a
// self-signed storage host silently disabled certificate verification on the
// credential exchange itself, letting anyone able to forge a certificate for
// login.microsoftonline.com harvest the tenant's client_secret and return a
// token of their choosing.
//
// azidentity owns its pipeline and takes no client from us, so this property is
// now structural rather than maintained by hand. The test stays: it pins the
// property against any future revision that reintroduces a client seam. A TLS
// server with a self-signed certificate stands in for the forged endpoint.
func TestTokenExchangeRefusesForgedCertificate(t *testing.T) {
	const secret = "SUPER-SECRET-VALUE"
	srv := tokenEndpoint(t, nil)
	defer srv.Close()
	// trustCert=false: the server stands in for a forged login.microsoftonline.com.
	pinAuthority(t, srv, false)

	tok, err := ClientCredentialsToken(context.Background(), "tenant-id", "client-id", secret, "https://graph.microsoft.com/.default")
	if err == nil {
		t.Fatalf("token exchange ACCEPTED an untrusted certificate and returned %q — the client secret is exposed to MITM", tok)
	}
	if !strings.Contains(err.Error(), "certificate") && !strings.Contains(err.Error(), "x509") {
		t.Errorf("expected a certificate-verification failure, got: %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("the client secret leaked into the error string: %v", err)
	}
}

// TestTokenExchangeIsCachedAcrossCalls proves the claim the code relies on:
// azidentity caches tokens internally, so caching the credential is enough and
// a flow calling twenty Azure actions performs one exchange, not twenty.
func TestTokenExchangeIsCachedAcrossCalls(t *testing.T) {
	var exchanges int
	srv := tokenEndpoint(t, &exchanges)
	defer srv.Close()
	pinAuthority(t, srv, true)

	for i := 0; i < 5; i++ {
		tok, err := ClientCredentialsToken(context.Background(), "t", "c", "sekret-value", "https://storage.azure.com/.default")
		if err != nil {
			t.Fatalf("call %d: %v", i, err)
		}
		if tok != "TOK" {
			t.Fatalf("token = %q", tok)
		}
	}
	if exchanges != 1 {
		t.Errorf("token endpoint hit %d times, want 1 — azidentity is expected to cache", exchanges)
	}
}

// TestTokenExchangeSeparatesPrincipals proves the credential cache cannot hand
// one principal's credential to another.
func TestTokenExchangeSeparatesPrincipals(t *testing.T) {
	srv := tokenEndpoint(t, nil)
	defer srv.Close()
	pinAuthority(t, srv, true)

	if _, err := ClientCredentialsToken(context.Background(), "t1", "c1", "sekret-one", "scope/.default"); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := ClientCredentialsToken(context.Background(), "t2", "c2", "sekret-two", "scope/.default"); err != nil {
		t.Fatalf("second: %v", err)
	}

	credCache.mu.Lock()
	defer credCache.mu.Unlock()
	if len(credCache.m) != 2 {
		t.Fatalf("cached credentials = %d, want 2 — distinct principals must not share one", len(credCache.m))
	}
	if credCache.m["t1|c1"] == credCache.m["t2|c2"] {
		t.Error("two principals resolved to the same credential")
	}
}

// A bad secret must surface Entra's own diagnosis — the AADSTS code names the
// real problem — without echoing the secret into the flow's error output.
func TestTokenExchangeSurfacesAADErrorWithoutLeakingSecret(t *testing.T) {
	const secret = "the-actual-secret-value"
	var srv *httptest.Server
	srv = httptest.NewUnstartedServer(nil)
	base := "https://" + srv.Listener.Addr().String()
	srv.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/.well-known/openid-configuration") {
			_, _ = w.Write([]byte(`{
				"issuer": "` + base + `/tenant/v2.0",
				"authorization_endpoint": "` + base + `/tenant/oauth2/v2.0/authorize",
				"token_endpoint": "` + base + `/tenant/oauth2/v2.0/token"
			}`))
			return
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"AADSTS7000215: Invalid client secret provided."}`))
	})
	srv.StartTLS()
	defer srv.Close()
	pinAuthority(t, srv, true)

	_, err := ClientCredentialsToken(context.Background(), "t", "c", secret, "sc/.default")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "AADSTS7000215") {
		t.Errorf("error should name the real problem, got: %v", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("client secret leaked into the error: %v", err)
	}
}

func TestClientCredentialsTokenValidatesInputs(t *testing.T) {
	cases := []struct{ tenant, client, secret, scope, wantErr string }{
		{"", "c", "s", "sc", "tenant ID is required"},
		{"t", "", "s", "sc", "client ID is required"},
		{"t", "c", "", "sc", "client secret is required"},
		{"bad/tenant", "c", "s", "sc", "invalid characters"},
		{"t", "c", "s", "", "a scope is required"},
	}
	for _, c := range cases {
		_, err := ClientCredentialsToken(context.Background(), c.tenant, c.client, c.secret, c.scope)
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("tenant=%q client=%q secret=%q scope=%q => %v, want %q", c.tenant, c.client, c.secret, c.scope, err, c.wantErr)
		}
	}
}

// TestMintTakesNoClient is the structural half of the MITM guard: the exchange
// must not grow a parameter that lets a caller supply transport again. A
// compile-time check is worth more than a runtime one here — the old signature
// was the bug.
func TestMintTakesNoClient(t *testing.T) {
	var f func(context.Context, string, string, string, string) (string, error) = ClientCredentialsToken
	_ = f
	if _, ok := interface{}(ClientCredentialsToken).(func(context.Context, *http.Client, string, string, string, string) (string, error)); ok {
		t.Fatal("ClientCredentialsToken accepts an *http.Client again — callers must never choose the transport for a credential exchange")
	}
}
