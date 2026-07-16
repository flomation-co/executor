// Package azure holds what every Azure service node shares: the Microsoft
// Entra ID client-credentials token exchange. Storage, Cosmos DB, and Entra
// directory actions all authenticate service principals against the same
// login.microsoftonline.com endpoint — only the requested scope differs — so
// the exchange and its per-execution cache live here, once.
//
// The service-specific clients (SharedKey signing, Cosmos master-key signing,
// Graph plumbing) stay in their own sub-packages; this package must not grow
// service knowledge.
package azure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// maxTokenResponse bounds the token endpoint's reply. A real token response is
// a few KB; this caps a hostile or misconfigured endpoint.
const maxTokenResponse = 1 << 20 // 1 MB

type cachedToken struct {
	token     string
	expiresAt time.Time
}

// tokenCache holds minted tokens for the lifetime of one execution. The
// executor is a one-shot process (one flow run, then exit), so this cache
// cannot accrete across time — it exists so a flow that calls twenty Azure
// actions performs one token exchange, not twenty. Modelled on the Shopify
// client-credentials cache (actions/ecommerce/shopify/common.go): mutex-guarded,
// proactive expiry buffer, pruned and bounded on the write path.
var tokenCache = struct {
	mu sync.Mutex
	m  map[string]cachedToken
}{m: map[string]cachedToken{}}

// maxCachedTokens bounds the cache. It only bites in a pathological run
// touching hundreds of distinct tenant|client|scope triples; a backstop, not a
// steady-state eviction policy.
const maxCachedTokens = 512

// ClientCredentialsToken returns a bearer token for the given service
// principal and scope, minting one via the OAuth2 client-credentials grant on
// first use and serving it from the per-execution cache afterwards.
//
// scope is the resource default scope, e.g. "https://graph.microsoft.com/.default"
// or "https://storage.azure.com/.default".
func ClientCredentialsToken(ctx context.Context, client *http.Client, tenantID, clientID, clientSecret, scope string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	clientID = strings.TrimSpace(clientID)
	if tenantID == "" {
		return "", fmt.Errorf("tenant ID is required for Microsoft Entra authentication")
	}
	if clientID == "" {
		return "", fmt.Errorf("client ID is required for Microsoft Entra authentication")
	}
	if clientSecret == "" {
		return "", fmt.Errorf("client secret is required for Microsoft Entra authentication")
	}
	// The tenant is interpolated into the token URL path; a GUID or a
	// *.onmicrosoft.com domain are both valid, but path or scheme
	// metacharacters are not.
	if strings.ContainsAny(tenantID, "/\\?#@ ") {
		return "", fmt.Errorf("tenant ID %q contains invalid characters", tenantID)
	}

	key := tenantID + "|" + clientID + "|" + scope
	tokenCache.mu.Lock()
	if c, ok := tokenCache.m[key]; ok && time.Now().Before(c.expiresAt) {
		tokenCache.mu.Unlock()
		return c.token, nil
	}
	tokenCache.mu.Unlock()

	// Mint outside the lock so a slow token endpoint doesn't serialise other
	// tenants; a concurrent double-mint is harmless (same credentials).
	tok, expiresIn, err := mintToken(ctx, client, tenantID, clientID, clientSecret, scope)
	if err != nil {
		return "", err
	}

	ttl := time.Duration(expiresIn) * time.Second
	if ttl <= 0 {
		ttl = 55 * time.Minute // Entra omitted a lifetime; tokens are typically 60-90 min.
	}
	// Expire the cache entry a margin before the real deadline so a cached
	// token is never handed out about to die mid-request. 5 minutes for the
	// usual ~1h grant, capped at half the lifetime for short-lived tokens.
	buffer := 5 * time.Minute
	if buffer > ttl/2 {
		buffer = ttl / 2
	}
	ttl -= buffer

	tokenCache.mu.Lock()
	pruneExpiredTokens()
	tokenCache.m[key] = cachedToken{token: tok, expiresAt: time.Now().Add(ttl)}
	tokenCache.mu.Unlock()
	return tok, nil
}

// pruneExpiredTokens drops timed-out entries and enforces maxCachedTokens.
// The caller must hold tokenCache.mu.
func pruneExpiredTokens() {
	now := time.Now()
	for k, v := range tokenCache.m {
		if !now.Before(v.expiresAt) {
			delete(tokenCache.m, k)
		}
	}
	for len(tokenCache.m) >= maxCachedTokens {
		for k := range tokenCache.m {
			delete(tokenCache.m, k)
			break
		}
	}
}

// tokenURL is a var so tests can point the exchange at an httptest server.
var tokenURL = "https://login.microsoftonline.com/%s/oauth2/v2.0/token"

func mintToken(ctx context.Context, client *http.Client, tenantID, clientID, clientSecret, scope string) (string, int, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)
	form.Set("scope", scope)

	endpoint := fmt.Sprintf(tokenURL, url.PathEscape(tenantID))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("failed to build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("Microsoft Entra token request failed: %s", RedactSecret(err.Error(), clientSecret))
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, maxTokenResponse))

	if resp.StatusCode != http.StatusOK {
		// AADSTS error bodies are JSON {error, error_description}; surface the
		// description, which names the actual problem (bad secret, unknown
		// tenant, missing admin consent) without echoing credentials.
		var apiErr struct {
			Error            string `json:"error"`
			ErrorDescription string `json:"error_description"`
		}
		_ = json.Unmarshal(body, &apiErr)
		msg := apiErr.ErrorDescription
		if msg == "" {
			msg = apiErr.Error
		}
		if msg == "" {
			msg = string(body)
		}
		return "", 0, fmt.Errorf("Microsoft Entra token request failed (%d): %s", resp.StatusCode, RedactSecret(msg, clientSecret))
	}

	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", 0, fmt.Errorf("failed to parse Microsoft Entra token response: %w", err)
	}
	if tok.AccessToken == "" {
		return "", 0, fmt.Errorf("Microsoft Entra token response contained no access token")
	}
	return tok.AccessToken, tok.ExpiresIn, nil
}

// RedactSecret masks every occurrence of secret in msg. Service packages use
// it (directly or via their own redact helpers) to keep credentials out of
// error strings and logs.
func RedactSecret(msg, secret string) string {
	if secret == "" {
		return msg
	}
	return strings.ReplaceAll(msg, secret, "********")
}
