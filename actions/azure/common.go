// Package azure holds what every Azure service node shares: the Microsoft
// Entra ID client-credentials token exchange. Storage, Cosmos DB, and Entra
// directory actions all authenticate service principals against the same
// login.microsoftonline.com endpoint — only the requested scope differs — so
// the exchange and its per-execution cache live here, once.
//
// The exchange itself is azidentity's, not ours. An earlier revision
// hand-rolled the OAuth2 POST, and that code is exactly where a real security
// hole appeared: it accepted an *http.Client from its callers, two of the three
// service packages passed the client built from their allow_insecure checkbox,
// and so an operator opting a self-signed storage host out of TLS verification
// silently disabled certificate verification on the credential exchange to
// Microsoft. azidentity owns its own pipeline and cannot be handed a client at
// all, which is the point: the bug is unrepresentable rather than merely fixed.
// Azure's auth flow has more edge cases than are worth re-deriving.
//
// The service-specific clients (SharedKey signing, Cosmos master-key signing,
// Graph plumbing) stay in their own sub-packages; this package must not grow
// service knowledge.
package azure

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/cloud"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
)

// managedTokenCredential wraps a platform-minted bearer token (the value an
// action's ${credentials.X} input resolves to under the "Connect Azure" auth
// method) as an azcore.TokenCredential the ARM/SDK clients can consume — the
// counterpart to entraCredential, but with no secret in the executor at all.
//
// The token is already scoped (the connector minted it for the right resource),
// so opts.Scopes is ignored, exactly as entraCredential ignores a non-matching
// requested scope. Each flow run resolves a fresh token and the api's
// credential-refresh poller keeps it live, so a short static expiry floor is
// enough to satisfy the SDK's cache without it trying to refresh here.
type managedTokenCredential struct{ token string }

func (c managedTokenCredential) GetToken(_ context.Context, _ policy.TokenRequestOptions) (azcore.AccessToken, error) {
	// The bearer is fixed for the life of the run — this adapter holds no refresh
	// token and cannot mint a new one, so ExpiresOn is a synthetic floor to satisfy
	// the SDK's cache, not a real expiry. Mid-run refresh is out of scope for
	// Phase 1: runs are short relative to the ~1h ARM token lifetime, and the api
	// poller keeps the STORED credential fresh so the NEXT run resolves a live
	// token. A run that outlives the token would see ARM 401s — acceptable now;
	// true mid-run refresh (carry the refresh token into the executor) is Phase 2.
	return azcore.AccessToken{Token: c.token, ExpiresOn: time.Now().Add(5 * time.Minute)}, nil
}

// NewManagedTokenCredential adapts a resolved managed bearer token into an
// azcore.TokenCredential. Empty token is a caller error (validate first).
func NewManagedTokenCredential(token string) azcore.TokenCredential {
	return managedTokenCredential{token: token}
}

// authorityHost is where the token exchange happens, fed to azidentity as a
// cloud configuration rather than used to build a URL by hand. It is a var as
// a test seam; production never changes it.
var authorityHost = cloud.AzurePublic.ActiveDirectoryAuthorityHost

// disableInstanceDiscovery turns off MSAL's authority validation. It is FALSE
// in production and must stay that way: with discovery on, azidentity asks
// Microsoft whether the authority is a real one before sending anything to it,
// which is a check the hand-rolled exchange never had — it would happily POST
// the client secret at whatever host it was given. Only the tests set this, and
// only because a local httptest server is by definition not a known Microsoft
// instance.
var disableInstanceDiscovery = false

// credTransport overrides azidentity's HTTP transport. It is nil in production
// — azcore supplies its own verifying transport — and is only ever set by this
// package's own tests, which need their httptest server's certificate trusted
// to exercise the success path (azidentity refuses a plain-HTTP authority, and
// rightly refuses a self-signed one).
//
// This is NOT the seam that caused the MITM bug and must not become it: that
// one was an exported *http.Client PARAMETER, so the storage and cosmosdb
// packages could — and did — hand their insecure client to a credential
// exchange. This is package-private and unreachable from any service package.
var credTransport policy.Transporter

// credCache holds one credential per tenant|client for the lifetime of the
// execution.
//
// The credential is what is cached, not the token: azidentity caches and
// refreshes tokens internally, per scope, so a flow that calls twenty Azure
// actions performs one token exchange rather than twenty (proved by
// TestTokenExchangeIsCachedAcrossCalls). The executor is a one-shot process —
// one flow run, then exit — so this cannot accrete across time; it is bounded
// anyway against a pathological run touching hundreds of distinct principals.
var credCache = struct {
	mu sync.Mutex
	m  map[string]*azidentity.ClientSecretCredential
}{m: map[string]*azidentity.ClientSecretCredential{}}

// maxCachedCredentials bounds the cache. A backstop for a single run that
// loops over hundreds of service principals, not a steady-state eviction
// policy.
const maxCachedCredentials = 512

// ClientCredentialsToken returns a bearer token for the given service
// principal and scope.
//
// scope is the resource default scope, e.g. "https://graph.microsoft.com/.default"
// or "https://storage.azure.com/.default".
func ClientCredentialsToken(ctx context.Context, tenantID, clientID, clientSecret, scope string) (string, error) {
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
	// azidentity validates the tenant itself, but its message is about its own
	// API; this one names the field the operator filled in.
	if strings.ContainsAny(tenantID, "/\\?#@ ") {
		return "", fmt.Errorf("tenant ID %q contains invalid characters", tenantID)
	}
	if scope == "" {
		return "", fmt.Errorf("a scope is required for Microsoft Entra authentication")
	}

	cred, err := credentialFor(tenantID, clientID, clientSecret)
	if err != nil {
		return "", fmt.Errorf("Microsoft Entra authentication failed: %s", RedactSecret(err.Error(), clientSecret))
	}

	tok, err := cred.GetToken(ctx, policy.TokenRequestOptions{Scopes: []string{scope}})
	if err != nil {
		// azidentity's error already carries the AADSTS code and description —
		// the part that names the real problem (bad secret, unknown tenant,
		// missing admin consent). Redact anyway: this string reaches
		// ErrorResult output and the execution log.
		return "", fmt.Errorf("Microsoft Entra token request failed: %s", RedactSecret(err.Error(), clientSecret))
	}
	if tok.Token == "" {
		return "", fmt.Errorf("Microsoft Entra returned an empty access token")
	}
	return tok.Token, nil
}

func credentialFor(tenantID, clientID, clientSecret string) (*azidentity.ClientSecretCredential, error) {
	key := tenantID + "|" + clientID

	credCache.mu.Lock()
	defer credCache.mu.Unlock()
	if c, ok := credCache.m[key]; ok {
		return c, nil
	}

	cred, err := azidentity.NewClientSecretCredential(tenantID, clientID, clientSecret,
		&azidentity.ClientSecretCredentialOptions{
			DisableInstanceDiscovery: disableInstanceDiscovery,
			ClientOptions: azcore.ClientOptions{
				Cloud:     cloud.Configuration{ActiveDirectoryAuthorityHost: authorityHost},
				Transport: credTransport,
			},
		})
	if err != nil {
		return nil, err
	}

	// Evict arbitrarily at the bound: these are interchangeable and a dropped
	// entry simply rebuilds on next use.
	for len(credCache.m) >= maxCachedCredentials {
		for k := range credCache.m {
			delete(credCache.m, k)
			break
		}
	}
	credCache.m[key] = cred
	return cred, nil
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
