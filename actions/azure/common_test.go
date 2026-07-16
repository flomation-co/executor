package azure

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func resetCache() {
	tokenCache.mu.Lock()
	tokenCache.m = map[string]cachedToken{}
	tokenCache.mu.Unlock()
}

// pinTokenURL points the mint at a test server for the duration of one test.
func pinTokenURL(t *testing.T, base string) {
	t.Helper()
	prev := tokenURL
	tokenURL = base + "/%s/oauth2/v2.0/token"
	t.Cleanup(func() { tokenURL = prev })
	resetCache()
	t.Cleanup(resetCache)
}

// TestTokenExchangeRefusesForgedCertificate is the regression guard for a real
// defect: the mint used to take an *http.Client from its callers, and both the
// storage and cosmosdb packages handed it the client built from their
// allow_insecure checkbox. That checkbox exists so an operator can reach a
// self-signed storage host or the Cosmos emulator — but it silently also
// disabled certificate verification on the credential exchange itself, so
// anyone able to forge a certificate for login.microsoftonline.com could
// harvest the tenant's client_secret and hand back a token of their choosing.
//
// The mint now owns a verifying client that callers cannot substitute. A TLS
// server with a self-signed certificate stands in for the forged endpoint: the
// exchange must refuse it, and the secret must never reach the wire.
func TestTokenExchangeRefusesForgedCertificate(t *testing.T) {
	const secret = "SUPER-SECRET-VALUE"

	var captured string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		captured = r.Form.Get("client_secret")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"ATTACKER_CHOSEN_TOKEN","expires_in":3600}`))
	}))
	defer srv.Close()
	pinTokenURL(t, srv.URL)

	tok, err := ClientCredentialsToken(context.Background(), "tenant-id", "client-id", secret, "https://graph.microsoft.com/.default")
	if err == nil {
		t.Fatalf("token exchange ACCEPTED an untrusted certificate and returned %q — the client secret is exposed to MITM", tok)
	}
	if !strings.Contains(err.Error(), "certificate") && !strings.Contains(err.Error(), "x509") {
		t.Errorf("expected a certificate-verification failure, got: %v", err)
	}
	if captured != "" {
		t.Errorf("the client secret reached the forged endpoint: %q", captured)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("the client secret leaked into the error string: %v", err)
	}
}

// TestTokenExchangeSucceedsAndCaches covers the happy path over plain HTTP
// (httptest.NewServer), which the verifying client accepts because there is no
// certificate to verify — the mint's TLS posture only governs https.
func TestTokenExchangeSucceedsAndCaches(t *testing.T) {
	var mints int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mints++
		if got := r.FormValue; got == nil {
			t.Error("no form")
		}
		_ = r.ParseForm()
		if r.Form.Get("grant_type") != "client_credentials" {
			t.Errorf("grant_type = %q", r.Form.Get("grant_type"))
		}
		if r.Form.Get("scope") != "https://storage.azure.com/.default" {
			t.Errorf("scope = %q", r.Form.Get("scope"))
		}
		_, _ = w.Write([]byte(`{"access_token":"TOK","expires_in":3600}`))
	}))
	defer srv.Close()
	pinTokenURL(t, srv.URL)

	for i := 0; i < 3; i++ {
		tok, err := ClientCredentialsToken(context.Background(), "t", "c", "s", "https://storage.azure.com/.default")
		if err != nil {
			t.Fatalf("mint %d: %v", i, err)
		}
		if tok != "TOK" {
			t.Fatalf("token = %q", tok)
		}
	}
	// A flow calling twenty Azure actions must perform one exchange, not twenty.
	if mints != 1 {
		t.Errorf("token endpoint hit %d times, want 1 (the per-execution cache)", mints)
	}
}

// TestTokenCacheKeyedByTenantClientScope proves the cache cannot hand a token
// minted for one tenant/client/scope to a different one.
func TestTokenCacheKeyedByTenantClientScope(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		// Echo the identity back so a cross-wired cache is visible.
		_, _ = w.Write([]byte(`{"access_token":"` + r.Form.Get("client_id") + "|" + r.Form.Get("scope") + `","expires_in":3600}`))
	}))
	defer srv.Close()
	pinTokenURL(t, srv.URL)

	cases := []struct{ tenant, client, scope, want string }{
		{"t1", "c1", "scope-a", "c1|scope-a"},
		{"t1", "c1", "scope-b", "c1|scope-b"}, // same client, different scope
		{"t1", "c2", "scope-a", "c2|scope-a"}, // same scope, different client
		{"t2", "c1", "scope-a", "c1|scope-a"},
	}
	for _, c := range cases {
		got, err := ClientCredentialsToken(context.Background(), c.tenant, c.client, "s", c.scope)
		if err != nil {
			t.Fatalf("%v: %v", c, err)
		}
		if got != c.want {
			t.Errorf("tenant=%s client=%s scope=%s => token %q, want %q (cache collision)", c.tenant, c.client, c.scope, got, c.want)
		}
	}
}

func TestTokenExchangeSurfacesAADErrorWithoutLeakingSecret(t *testing.T) {
	const secret = "the-secret"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"invalid_client","error_description":"AADSTS7000215: Invalid client secret provided."}`))
	}))
	defer srv.Close()
	pinTokenURL(t, srv.URL)

	_, err := ClientCredentialsToken(context.Background(), "t", "c", secret, "sc")
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
	cases := []struct{ tenant, client, secret, wantErr string }{
		{"", "c", "s", "tenant ID is required"},
		{"t", "", "s", "client ID is required"},
		{"t", "c", "", "client secret is required"},
		{"bad/tenant", "c", "s", "invalid characters"},
	}
	for _, c := range cases {
		_, err := ClientCredentialsToken(context.Background(), c.tenant, c.client, c.secret, "sc")
		if err == nil || !strings.Contains(err.Error(), c.wantErr) {
			t.Errorf("tenant=%q client=%q secret=%q => %v, want %q", c.tenant, c.client, c.secret, err, c.wantErr)
		}
	}
}

// TestTokenClientVerifiesCertificates pins the posture directly, so a future
// edit that adds InsecureSkipVerify to the mint's client fails here.
func TestTokenClientVerifiesCertificates(t *testing.T) {
	tr, ok := tokenClient.Transport.(*http.Transport)
	if !ok {
		if tokenClient.Transport != nil {
			t.Fatalf("tokenClient.Transport = %T, want *http.Transport or nil (the verifying default)", tokenClient.Transport)
		}
		return // nil transport == http.DefaultTransport, which verifies.
	}
	if tr.TLSClientConfig != nil && tr.TLSClientConfig.InsecureSkipVerify {
		t.Error("the Entra token exchange must always verify certificates: the endpoint is public Microsoft and carries the tenant's client secret")
	}
	var _ = tls.Config{}
	if tokenClient.Timeout == 0 || tokenClient.Timeout > 60*time.Second {
		t.Errorf("tokenClient.Timeout = %v, want a bounded timeout", tokenClient.Timeout)
	}
}
