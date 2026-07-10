// Package kubernetes holds the shared REST client, auth resolution, and generic
// object CRUD used by every infrastructure/kubernetes/* action — and, for the
// Helm release-secret reads, by infrastructure/helm/* too.
//
// It speaks to the Kubernetes API server directly over HTTPS with net/http,
// rather than through k8s.io/client-go. That is a deliberate trade: client-go
// (and the Helm SDK that depends on it) would add tens of megabytes to a binary
// the runner spawns once per flow execution, and would fold the whole k8s.io
// advisory tree into this repo's govulncheck lint gate — where a new CVE in a
// package we never call would start failing unrelated branches. The Kubernetes
// REST API is stable, versioned, and entirely adequate to address from stdlib.
//
// Three things shape this file:
//
//   - Errors come back as a Status object ({"kind":"Status","message","reason",
//     "code"}), not as a bare body, so CheckResponse decodes it for a message an
//     operator can act on ("pods 'x' not found" beats "404").
//   - Lists paginate with an opaque `continue` token echoed in metadata.continue,
//     not with page numbers. An empty token means the collection is exhausted.
//   - TLS material varies per connection (a pasted CA, a client certificate),
//     so the http.Client cannot be a single package-level value the way it is for
//     WordPress or Jenkins. Clients are built once per distinct TLS
//     configuration and cached, keyed by a fingerprint of that material.
package kubernetes

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	yaml "go.yaml.in/yaml/v3"

	core "flomation.app/automate/executor"
)

const (
	// maxResponseBody caps a response body. Pod logs are the pathological case —
	// a chatty container can emit gigabytes — so reads are bounded and the action
	// reports truncation rather than exhausting the runner's memory.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout bounds a single API call. Generous because log reads and
	// large list pages over a slow link are legitimately slow.
	requestTimeout = 60 * time.Second

	// DefaultListLimit / MaxListLimit bound the per-page `limit` query parameter.
	// Kubernetes itself imposes no ceiling; these keep a single response inside
	// maxResponseBody for realistic object sizes.
	DefaultListLimit = 100
	MaxListLimit     = 500

	// MaxAllPages bounds a "return all" pagination walk so a cluster with a huge
	// collection can never spin unbounded requests. On hitting the cap the action
	// returns the continue token so the caller can resume.
	MaxAllPages = 100
)

// Auth methods, as stored in the auth_method input.
const (
	AuthMethodToken      = "token"
	AuthMethodClientCert = "client_cert"
	AuthMethodKubeconfig = "kubeconfig"
)

// Auth is the resolved connection to an API server: where it is, how to prove
// who we are, and how to verify (or knowingly not verify) its certificate.
type Auth struct {
	// Server is the normalised API server URL: scheme://host[:port], no path,
	// no trailing slash.
	Server string
	// Token is a bearer token (a ServiceAccount token). Empty for mTLS.
	Token string
	// CACert is the PEM-encoded CA bundle that signs the API server's
	// certificate. Empty means "use the system roots".
	CACert []byte
	// ClientCert / ClientKey are the PEM-encoded client keypair for mTLS.
	ClientCert []byte
	ClientKey  []byte
	// Insecure skips API server certificate verification entirely. Opt-in, for
	// self-signed clusters where the operator has no CA to hand.
	Insecure bool
}

// APIResponse is a raw response, kept undecoded so callers can branch on status
// before paying to parse (and so pod logs, which are text/plain, need no JSON).
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
	// Truncated reports that the body hit maxResponseBody and is incomplete.
	Truncated bool
}

// ---------------------------------------------------------------------------
// Shared input declarations
// ---------------------------------------------------------------------------

// AuthInputs is the canonical credential block. Action packages re-declare their
// own literal Inputs arrays (the manifest generator AST-parses those literals,
// so it cannot see through a shared variable), but every action puts these
// first, in this order.
//
// The names matter. core.FindConnection returns the FIRST input whose name
// matches, and auth inputs are declared first — so an auth field that shares a
// name with a resource field silently shadows it, and the action reads the
// credential where it meant to read the resource. Kubernetes is unusually
// exposed to this: `token`, `name`, `namespace`, `ca`, `cert` and `key` are all
// plausible resource fields. Hence api_server_url (not url),
// service_account_token (not token), cluster_ca_cert (not ca), and no auth-level
// namespace whatsoever.
var AuthInputs = []core.Connection{
	{
		Name:        "api_server_url",
		Type:        core.ConnectionTypeString,
		Label:       "API Server URL",
		Placeholder: "https://your-cluster:6443 — the Kubernetes API endpoint",
		Required:    true,
	},
	{
		Name:  "auth_method",
		Type:  core.ConnectionTypeString,
		Label: "Authentication",
		Options: []core.ConnectionOption{
			{Name: "Service Account Token", Value: AuthMethodToken},
			{Name: "Client Certificate (mTLS)", Value: AuthMethodClientCert},
			{Name: "Kubeconfig (paste)", Value: AuthMethodKubeconfig},
		},
	},
	{
		// NOT "token" — a resource field named token would resolve to this.
		Name:        "service_account_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Service Account Token",
		Placeholder: "kubectl create token <serviceaccount> -n <namespace>",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"", AuthMethodToken}},
	},
	{
		Name:        "cluster_ca_cert",
		Type:        core.ConnectionTypeText,
		Label:       "Cluster CA Certificate (PEM)",
		Placeholder: "-----BEGIN CERTIFICATE----- … Leave blank to use the system trust store",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"", AuthMethodToken, AuthMethodClientCert}},
	},
	{
		Name:        "client_certificate",
		Type:        core.ConnectionTypeSecret,
		Label:       "Client Certificate (PEM)",
		Placeholder: "-----BEGIN CERTIFICATE-----",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthMethodClientCert}},
	},
	{
		Name:        "client_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "Client Key (PEM)",
		Placeholder: "-----BEGIN RSA PRIVATE KEY-----",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthMethodClientCert}},
	},
	{
		Name:        "kubeconfig",
		Type:        core.ConnectionTypeSecret,
		Label:       "Kubeconfig YAML",
		Placeholder: "Paste the full kubeconfig; the current-context is used",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthMethodKubeconfig}},
	},
	{
		Name:        "allow_insecure",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Allow Insecure TLS",
		Placeholder: "Skip API server certificate verification — only for self-signed clusters with no CA to hand",
	},
}

// ConfirmDestructiveInput is the guard every destructive action carries. It is a
// boolean so the editor renders a checkbox, but the editor also offers a
// variable picker on booleans — so a flow can bind it to ${var.approved} or to
// an upstream node's boolean output and gate the deletion on a condition
// evaluated at run time, rather than on a box someone ticked at design time.
//
// See ConfirmDestructive for why the value is read without Connection.Boolean().
var ConfirmDestructiveInput = core.Connection{
	Name:        "confirm_destructive",
	Type:        core.ConnectionTypeBoolean,
	Label:       "Confirm Destructive Action",
	Placeholder: "This permanently changes cluster state. Tick to allow, or bind a variable such as ${var.approved}",
	Required:    true,
}

// ---------------------------------------------------------------------------
// Auth resolution
// ---------------------------------------------------------------------------

// GetAuth resolves an Auth from an action's credential inputs, dispatching on
// auth_method. A missing or malformed credential is a hard error: there is
// nothing sensible to attempt against an API server without one.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	method := OptionalString("auth_method", inputs)
	if method == "" {
		method = AuthMethodToken
	}

	a := Auth{Insecure: BoolInput("allow_insecure", inputs)}

	switch method {
	case AuthMethodToken:
		server, err := RequiredString("api_server_url", inputs)
		if err != nil {
			return Auth{}, err
		}
		if a.Server, err = NormaliseServerURL(server); err != nil {
			return Auth{}, err
		}
		if a.Token, err = RequiredString("service_account_token", inputs); err != nil {
			return Auth{}, err
		}
		a.CACert = []byte(OptionalString("cluster_ca_cert", inputs))

	case AuthMethodClientCert:
		server, err := RequiredString("api_server_url", inputs)
		if err != nil {
			return Auth{}, err
		}
		if a.Server, err = NormaliseServerURL(server); err != nil {
			return Auth{}, err
		}
		cert, err := RequiredString("client_certificate", inputs)
		if err != nil {
			return Auth{}, err
		}
		key, err := RequiredString("client_key", inputs)
		if err != nil {
			return Auth{}, err
		}
		a.ClientCert = []byte(cert)
		a.ClientKey = []byte(key)
		a.CACert = []byte(OptionalString("cluster_ca_cert", inputs))

	case AuthMethodKubeconfig:
		raw, err := RequiredString("kubeconfig", inputs)
		if err != nil {
			return Auth{}, err
		}
		a, err = parseKubeconfig([]byte(raw))
		if err != nil {
			return Auth{}, err
		}
		// An explicit URL overrides the kubeconfig's server, for the case where
		// the file was written for an address only reachable from elsewhere.
		if override := OptionalString("api_server_url", inputs); override != "" {
			if a.Server, err = NormaliseServerURL(override); err != nil {
				return Auth{}, err
			}
		}
		// The action's own checkbox can force insecure on top of the file.
		if BoolInput("allow_insecure", inputs) {
			a.Insecure = true
		}

	default:
		return Auth{}, fmt.Errorf("auth_method must be one of %q, %q or %q (got %q)",
			AuthMethodToken, AuthMethodClientCert, AuthMethodKubeconfig, method)
	}

	if len(a.CACert) > 0 && !bytes.Contains(a.CACert, []byte("BEGIN CERTIFICATE")) {
		return Auth{}, fmt.Errorf("cluster_ca_cert does not look like a PEM certificate — it should start with -----BEGIN CERTIFICATE-----")
	}
	return a, nil
}

// NormaliseServerURL reduces whatever was pasted to scheme://host[:port],
// defaulting to https (the API server never serves plaintext in a real cluster)
// and stripping any path, query, or embedded credentials.
func NormaliseServerURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("api_server_url is required")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("api_server_url is not a valid URL: %w", err)
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", fmt.Errorf("api_server_url must be an http(s) URL, e.g. https://your-cluster:6443")
	}
	if u.Host == "" {
		return "", fmt.Errorf("api_server_url must include a host, e.g. https://your-cluster:6443")
	}
	return u.Scheme + "://" + u.Host, nil
}

// kubeconfigFile is the subset of a kubeconfig this node understands. Fields it
// cannot honour (exec plugins, auth-provider) are parsed only so they can be
// rejected with an explanation rather than a confusing 401.
type kubeconfigFile struct {
	CurrentContext string `yaml:"current-context"`
	Clusters       []struct {
		Name    string `yaml:"name"`
		Cluster struct {
			Server                   string `yaml:"server"`
			CertificateAuthorityData string `yaml:"certificate-authority-data"`
			CertificateAuthority     string `yaml:"certificate-authority"`
			InsecureSkipTLSVerify    bool   `yaml:"insecure-skip-tls-verify"`
		} `yaml:"cluster"`
	} `yaml:"clusters"`
	Contexts []struct {
		Name    string `yaml:"name"`
		Context struct {
			Cluster string `yaml:"cluster"`
			User    string `yaml:"user"`
		} `yaml:"context"`
	} `yaml:"contexts"`
	Users []struct {
		Name string `yaml:"name"`
		User struct {
			Token                 string                 `yaml:"token"`
			ClientCertificateData string                 `yaml:"client-certificate-data"`
			ClientKeyData         string                 `yaml:"client-key-data"`
			ClientCertificate     string                 `yaml:"client-certificate"`
			ClientKey             string                 `yaml:"client-key"`
			Exec                  map[string]interface{} `yaml:"exec"`
			AuthProvider          map[string]interface{} `yaml:"auth-provider"`
		} `yaml:"user"`
	} `yaml:"users"`
}

// parseKubeconfig resolves the current-context's cluster and user into an Auth.
//
// Only inline (…-data) credentials are honoured. A kubeconfig that points at
// files on disk, or that shells out to a cloud CLI via an exec plugin, describes
// an environment the executor does not have; saying so plainly beats letting the
// request fail later as an opaque 401.
func parseKubeconfig(raw []byte) (Auth, error) {
	var kc kubeconfigFile
	if err := yaml.Unmarshal(raw, &kc); err != nil {
		return Auth{}, fmt.Errorf("kubeconfig is not valid YAML: %w", err)
	}
	if strings.TrimSpace(kc.CurrentContext) == "" {
		return Auth{}, fmt.Errorf("kubeconfig has no current-context set")
	}

	var clusterName, userName string
	for _, c := range kc.Contexts {
		if c.Name == kc.CurrentContext {
			clusterName, userName = c.Context.Cluster, c.Context.User
			break
		}
	}
	if clusterName == "" {
		return Auth{}, fmt.Errorf("kubeconfig has no context named %q", kc.CurrentContext)
	}

	var a Auth
	found := false
	for _, c := range kc.Clusters {
		if c.Name != clusterName {
			continue
		}
		found = true
		server, err := NormaliseServerURL(c.Cluster.Server)
		if err != nil {
			return Auth{}, fmt.Errorf("kubeconfig cluster %q: %w", clusterName, err)
		}
		a.Server = server
		a.Insecure = c.Cluster.InsecureSkipTLSVerify
		if c.Cluster.CertificateAuthorityData != "" {
			ca, err := base64.StdEncoding.DecodeString(c.Cluster.CertificateAuthorityData)
			if err != nil {
				return Auth{}, fmt.Errorf("kubeconfig cluster %q: certificate-authority-data is not valid base64: %w", clusterName, err)
			}
			a.CACert = ca
		} else if c.Cluster.CertificateAuthority != "" {
			return Auth{}, fmt.Errorf("kubeconfig cluster %q references a CA file on disk (certificate-authority); "+
				"paste the certificate inline as certificate-authority-data, or use the Cluster CA Certificate field", clusterName)
		}
		break
	}
	if !found {
		return Auth{}, fmt.Errorf("kubeconfig has no cluster named %q", clusterName)
	}

	for _, u := range kc.Users {
		if u.Name != userName {
			continue
		}
		switch {
		case len(u.User.Exec) > 0:
			return Auth{}, fmt.Errorf("kubeconfig user %q authenticates with an exec plugin, which needs a cloud CLI the executor does not have — "+
				"create a ServiceAccount token instead (kubectl create token)", userName)
		case len(u.User.AuthProvider) > 0:
			return Auth{}, fmt.Errorf("kubeconfig user %q uses a legacy auth-provider, which is not supported — "+
				"create a ServiceAccount token instead (kubectl create token)", userName)
		case u.User.Token != "":
			a.Token = u.User.Token
		case u.User.ClientCertificateData != "" && u.User.ClientKeyData != "":
			cert, err := base64.StdEncoding.DecodeString(u.User.ClientCertificateData)
			if err != nil {
				return Auth{}, fmt.Errorf("kubeconfig user %q: client-certificate-data is not valid base64: %w", userName, err)
			}
			key, err := base64.StdEncoding.DecodeString(u.User.ClientKeyData)
			if err != nil {
				return Auth{}, fmt.Errorf("kubeconfig user %q: client-key-data is not valid base64: %w", userName, err)
			}
			a.ClientCert, a.ClientKey = cert, key
		case u.User.ClientCertificate != "" || u.User.ClientKey != "":
			return Auth{}, fmt.Errorf("kubeconfig user %q references certificate files on disk; "+
				"paste them inline as client-certificate-data / client-key-data", userName)
		default:
			return Auth{}, fmt.Errorf("kubeconfig user %q has no token and no client certificate", userName)
		}
		break
	}
	if a.Token == "" && len(a.ClientCert) == 0 {
		return Auth{}, fmt.Errorf("kubeconfig has no usable credential for user %q", userName)
	}
	return a, nil
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// clientCache memoises one http.Client per distinct TLS configuration, so a flow
// running many actions against the same cluster pools its connections. The key
// covers only TLS material — the bearer token rides in a per-request header and
// does not affect the transport.
var (
	clientCacheMu sync.Mutex
	clientCache   = map[string]*http.Client{}
)

func tlsFingerprint(a Auth) string {
	h := sha256.New()
	h.Write(a.CACert)
	h.Write([]byte{0})
	h.Write(a.ClientCert)
	h.Write([]byte{0})
	h.Write(a.ClientKey)
	h.Write([]byte{0})
	if a.Insecure {
		h.Write([]byte("insecure"))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// clientFor builds (or reuses) the http.Client for a connection's TLS material.
func clientFor(a Auth) (*http.Client, error) {
	key := tlsFingerprint(a)

	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	if c, ok := clientCache[key]; ok {
		return c, nil
	}

	// #nosec G402 -- InsecureSkipVerify is opt-in per connection (allow_insecure),
	// for self-signed clusters. It is never the default, and a supplied CA is
	// preferred; the two are mutually exclusive because crypto/tls ignores
	// RootCAs once verification is skipped.
	tlsCfg := &tls.Config{MinVersion: tls.VersionTLS12}

	if a.Insecure {
		tlsCfg.InsecureSkipVerify = true
	} else if len(a.CACert) > 0 {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(a.CACert) {
			return nil, fmt.Errorf("cluster_ca_cert contains no usable PEM certificate")
		}
		tlsCfg.RootCAs = pool
	}

	if len(a.ClientCert) > 0 || len(a.ClientKey) > 0 {
		pair, err := tls.X509KeyPair(a.ClientCert, a.ClientKey)
		if err != nil {
			return nil, fmt.Errorf("client certificate and key do not form a valid keypair: %w", err)
		}
		tlsCfg.Certificates = []tls.Certificate{pair}
	}

	c := &http.Client{
		Timeout: requestTimeout,
		Transport: &http.Transport{
			MaxIdleConns:        50,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     90 * time.Second,
			TLSClientConfig:     tlsCfg,
		},
	}
	clientCache[key] = c
	return c, nil
}

// ResetClientCacheForTest drops every memoised client. Test-only: without it a
// test that swaps TLS material behind the same fingerprint would reuse a stale
// transport.
func ResetClientCacheForTest() {
	clientCacheMu.Lock()
	defer clientCacheMu.Unlock()
	clientCache = map[string]*http.Client{}
}

// ExecuteAPI performs one call against the API server.
//
// path must already be a rooted API path (see Resource.Path), optionally with a
// query string. contentType is empty for bodyless requests; accept defaults to
// application/json but is overridden for the /log subresource, which returns
// text/plain.
func ExecuteAPI(ctx context.Context, a Auth, method, path string, body []byte, contentType, accept string) (*APIResponse, error) {
	client, err := clientFor(a)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if len(body) > 0 {
		reader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, a.Server+path, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", Redact(a, err.Error()))
	}

	if a.Token != "" {
		req.Header.Set("Authorization", "Bearer "+a.Token)
	}
	if accept == "" {
		accept = "application/json"
	}
	req.Header.Set("Accept", accept)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Kubernetes API request failed: %s", Redact(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	// Read one byte past the cap so a body that exactly fills it is not
	// mislabelled as truncated.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %s", Redact(a, err.Error()))
	}
	truncated := false
	if len(raw) > maxResponseBody {
		raw, truncated = raw[:maxResponseBody], true
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: raw, Headers: resp.Header, Truncated: truncated}, nil
}

// Redact scrubs credential material from a string bound for an error message or
// a log line. A bearer token travels in a header, not a URL, so a leak is
// unlikely — but a proxy error or a wrapped transport error can echo one, and a
// token in a flow's error output is a token in the audit log.
func Redact(a Auth, msg string) string {
	if a.Token != "" {
		msg = strings.ReplaceAll(msg, a.Token, "REDACTED")
	}
	if len(a.ClientKey) > 0 {
		msg = strings.ReplaceAll(msg, string(a.ClientKey), "REDACTED")
	}
	return msg
}

// status is the Kubernetes error envelope, returned for every non-2xx.
type status struct {
	Kind    string `json:"kind"`
	Status  string `json:"status"`
	Message string `json:"message"`
	Reason  string `json:"reason"`
	Code    int    `json:"code"`
}

// CheckResponse verifies a 2xx, decoding the Status envelope into a message an
// operator can act on. Kubernetes' own message ("deployments.apps \"web\" not
// found") is far more useful than the status code, so it leads.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var st status
	if err := json.Unmarshal(resp.Body, &st); err == nil && st.Message != "" {
		if st.Reason != "" {
			return fmt.Errorf("Kubernetes API error (%d %s): %s", resp.StatusCode, st.Reason, st.Message)
		}
		return fmt.Errorf("Kubernetes API error (%d): %s", resp.StatusCode, st.Message)
	}

	body := strings.TrimSpace(string(resp.Body))
	if len(body) > 500 {
		body = body[:500]
	}
	if body == "" {
		body = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("Kubernetes API error (%d): %s", resp.StatusCode, body)
}

// Decode unmarshals a successful single-object body.
func Decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Kubernetes response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Generic object CRUD
// ---------------------------------------------------------------------------

// Get retrieves one object.
func Get(ctx context.Context, a Auth, r Resource, namespace, name string) (map[string]interface{}, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	path, err := r.Path(namespace, name)
	if err != nil {
		return nil, err
	}
	resp, err := ExecuteAPI(ctx, a, http.MethodGet, path, nil, "", "")
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return Decode(resp)
}

// Create POSTs a new object into a collection.
func Create(ctx context.Context, a Auth, r Resource, namespace string, obj map[string]interface{}) (map[string]interface{}, error) {
	path, err := r.Path(namespace, "")
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s: %w", r.Kind, err)
	}
	resp, err := ExecuteAPI(ctx, a, http.MethodPost, path, body, "application/json", "")
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return Decode(resp)
}

// Patch applies a partial update. patchType selects the merge semantic — see the
// Patch* constants in resources.go; picking the wrong one is a 415 at best.
func Patch(ctx context.Context, a Auth, r Resource, namespace, name string, patch []byte, patchType string) (map[string]interface{}, error) {
	path, err := r.Path(namespace, name)
	if err != nil {
		return nil, err
	}
	resp, err := ExecuteAPI(ctx, a, http.MethodPatch, path, patch, patchType, "")
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return Decode(resp)
}

// PatchSub applies a partial update to a subresource, e.g. /scale.
func PatchSub(ctx context.Context, a Auth, r Resource, namespace, name, sub string, patch []byte, patchType string) (map[string]interface{}, error) {
	path, err := r.SubPath(namespace, name, sub)
	if err != nil {
		return nil, err
	}
	resp, err := ExecuteAPI(ctx, a, http.MethodPatch, path, patch, patchType, "")
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return Decode(resp)
}

// DeleteOptions configures how the server tears an object down.
type DeleteOptions struct {
	// PropagationPolicy is Foreground, Background, or Orphan. Background is the
	// server default: the object goes immediately and its dependents are garbage
	// collected afterwards. Foreground blocks until dependents are gone.
	PropagationPolicy string
	// GracePeriodSeconds overrides the object's terminationGracePeriodSeconds.
	// A pointer so that 0 (delete immediately, no graceful shutdown) is
	// distinguishable from "unset".
	GracePeriodSeconds *int64
}

// Delete removes one object.
func Delete(ctx context.Context, a Auth, r Resource, namespace, name string, opts DeleteOptions) (map[string]interface{}, error) {
	if strings.TrimSpace(name) == "" {
		return nil, fmt.Errorf("name is required")
	}
	path, err := r.Path(namespace, name)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "DeleteOptions",
	}
	if opts.PropagationPolicy != "" {
		payload["propagationPolicy"] = opts.PropagationPolicy
	}
	if opts.GracePeriodSeconds != nil {
		payload["gracePeriodSeconds"] = *opts.GracePeriodSeconds
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	resp, err := ExecuteAPI(ctx, a, http.MethodDelete, path, body, "application/json", "")
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return Decode(resp)
}

// ListOptions are the query parameters shared by every collection read.
type ListOptions struct {
	LabelSelector string
	FieldSelector string
	Limit         int
	Continue      string
	// ReturnAll walks continue tokens until the collection is exhausted or
	// MaxAllPages is reached.
	ReturnAll bool
}

// Query renders the options as URL query parameters.
func (o ListOptions) Query() url.Values {
	q := url.Values{}
	if o.LabelSelector != "" {
		q.Set("labelSelector", o.LabelSelector)
	}
	if o.FieldSelector != "" {
		q.Set("fieldSelector", o.FieldSelector)
	}
	if o.Limit > 0 {
		q.Set("limit", strconv.Itoa(o.Limit))
	}
	if o.Continue != "" {
		q.Set("continue", o.Continue)
	}
	return q
}

// List reads a collection.
//
// Kubernetes paginates with an opaque continue token echoed at
// metadata.continue; an empty token means the collection is exhausted. When
// ReturnAll is set the walk follows those tokens up to MaxAllPages, and the
// token from the last page fetched is returned so a caller that hit the cap can
// resume rather than silently believing it saw everything.
//
// namespace may be "" for a namespaced kind, which reads across all namespaces
// (/api/v1/pods rather than /api/v1/namespaces/x/pods) — the shape kubectl calls
// --all-namespaces.
func List(ctx context.Context, a Auth, r Resource, namespace string, opts ListOptions) (items []interface{}, continueToken string, pages int, err error) {
	items = []interface{}{}

	var path string
	if r.Namespaced && strings.TrimSpace(namespace) == "" {
		path = r.APIRoot() + "/" + r.Plural // all namespaces
	} else if path, err = r.Path(namespace, ""); err != nil {
		return nil, "", 0, err
	}

	if opts.Limit <= 0 {
		opts.Limit = DefaultListLimit
	}
	if opts.Limit > MaxListLimit {
		opts.Limit = MaxListLimit
	}

	for {
		q := opts.Query()
		full := path
		if enc := q.Encode(); enc != "" {
			full += "?" + enc
		}

		resp, e := ExecuteAPI(ctx, a, http.MethodGet, full, nil, "", "")
		if e != nil {
			return nil, "", pages, e
		}
		if e := CheckResponse(resp); e != nil {
			return nil, "", pages, e
		}

		var page struct {
			Items    []interface{} `json:"items"`
			Metadata struct {
				Continue string `json:"continue"`
			} `json:"metadata"`
		}
		if e := json.Unmarshal(resp.Body, &page); e != nil {
			return nil, "", pages, fmt.Errorf("failed to parse Kubernetes list response: %w", e)
		}

		items = append(items, page.Items...)
		pages++
		continueToken = page.Metadata.Continue

		if !opts.ReturnAll || continueToken == "" || pages >= MaxAllPages {
			break
		}
		opts.Continue = continueToken
	}
	return items, continueToken, pages, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// OptionalString extracts a string input, returning "" when absent.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}

// RequiredString extracts a required string input, erroring when absent/blank.
func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// OptionalInt extracts an integer input. The bool is false when absent.
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil {
		return 0, false
	}
	if n := conn.Number(); n != nil {
		return int(*n), true
	}
	// An integer field carrying a resolved ${...} reference arrives as a string.
	if s := OptionalString(name, inputs); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			return n, true
		}
	}
	return 0, false
}

// BoolInput reads a boolean input, coercing a string value.
//
// It deliberately does NOT use core.Connection.Boolean(). That method asserts
// c.Value.(bool) and returns nil for anything else — but the editor's
// BooleanProperty stores a variable reference as a *string* ("${var.approved}"),
// and the flow engine's substitution pass rewrites every ${...} into a string
// before the action ever sees it. So a checkbox bound to a variable reaches
// Boolean() as the string "true" and reads back as unset.
//
// (Connection.String() and Connection.Number() already coerce this way; Boolean()
// is the odd one out. Fixing it is a platform change with a blast radius beyond
// this node, so it ships separately. Coercing here keeps these actions correct
// regardless of which lands first — the bool fast path below stays exact.)
func BoolInput(name string, inputs []*core.Connection) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return false
	}
	if b, ok := conn.Value.(bool); ok {
		return b
	}
	s := strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", conn.Value)))
	switch s {
	case "yes", "on":
		return true
	case "", "no", "off":
		return false
	}
	b, err := strconv.ParseBool(s)
	if err != nil {
		return false
	}
	return b
}

// ConfirmDestructive gates an action that permanently changes cluster state.
//
// It fails closed: an unset, blank, or unparseable value refuses the action. An
// unresolvable ${var.x} substitutes to the empty string, so a typo'd variable
// name declines to delete a namespace rather than deleting it.
func ConfirmDestructive(inputs []*core.Connection, what string) error {
	if BoolInput("confirm_destructive", inputs) {
		return nil
	}
	return fmt.Errorf("refusing to %s: tick “Confirm Destructive Action” on this node, "+
		"or bind it to a variable (e.g. ${var.approved}) that evaluates to true at run time", what)
}

// OptionalJSON parses an object-typed input. Returns (nil, nil) when absent.
func OptionalJSON(name string, inputs []*core.Connection) (interface{}, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return nil, nil
	}
	switch v := conn.Value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		var out interface{}
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, fmt.Errorf("%s must be valid JSON: %w", name, err)
		}
		return out, nil
	default:
		return conn.Value, nil
	}
}

// OptionalJSONObject parses an object-typed input, insisting it is a JSON object
// rather than an array or scalar. Returns (nil, nil) when absent.
func OptionalJSONObject(name string, inputs []*core.Connection) (map[string]interface{}, error) {
	v, err := OptionalJSON(name, inputs)
	if err != nil || v == nil {
		return nil, err
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a JSON object, e.g. {"key":"value"}`, name)
	}
	return obj, nil
}

// StringMapInput reads an input that should be a flat map of string→string —
// a ConfigMap's data, a Secret's stringData, an object's labels. Non-string
// scalars are stringified (a YAML-ish `replicas: 3` is a common paste), but a
// nested object or array is rejected: Kubernetes would reject it too, later and
// less clearly.
func StringMapInput(name string, inputs []*core.Connection) (map[string]string, error) {
	obj, err := OptionalJSONObject(name, inputs)
	if err != nil || obj == nil {
		return nil, err
	}
	out := make(map[string]string, len(obj))
	for k, v := range obj {
		switch tv := v.(type) {
		case string:
			out[k] = tv
		case bool:
			out[k] = strconv.FormatBool(tv)
		case float64:
			// JSON numbers decode to float64; render whole numbers without ".0".
			if tv == float64(int64(tv)) {
				out[k] = strconv.FormatInt(int64(tv), 10)
			} else {
				out[k] = strconv.FormatFloat(tv, 'f', -1, 64)
			}
		case nil:
			out[k] = ""
		default:
			return nil, fmt.Errorf("%s[%q] must be a string — nested objects and arrays are not valid here", name, k)
		}
	}
	return out, nil
}

// ListOptionsFrom reads the filter/pagination inputs every *_list action shares.
func ListOptionsFrom(inputs []*core.Connection) ListOptions {
	limit, _ := OptionalInt("limit", inputs)
	return ListOptions{
		LabelSelector: OptionalString("label_selector", inputs),
		FieldSelector: OptionalString("field_selector", inputs),
		Limit:         limit,
		Continue:      OptionalString("continue_token", inputs),
		ReturnAll:     BoolInput("return_all", inputs),
	}
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ObjectResult shapes a single-object response into the standard action output.
// The id output carries the object's name, which is what every other action in a
// flow needs to address it again — a Kubernetes UID is unique but unusable as an
// input anywhere else.
func ObjectResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          ObjectName(obj),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response. continueToken is non-empty when more
// objects remain — either because a single page was requested, or because a
// ReturnAll walk hit MaxAllPages.
func ListResult(items []interface{}, continueToken, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":        items,
		"count":          len(items),
		"continue_token": continueToken,
		"has_more":       continueToken != "",
		"tool_result":    summary,
		"success":        true,
		"error":          "",
	}
}

// ErrorResult is the standard soft-failure output map, returned alongside a nil
// error so the engine records it as data on the error port rather than aborting
// the flow.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ObjectName pulls metadata.name out of a decoded object, "" when absent.
func ObjectName(obj map[string]interface{}) string {
	md, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	name, _ := md["name"].(string)
	return name
}

// ObjectNamespace pulls metadata.namespace out of a decoded object.
func ObjectNamespace(obj map[string]interface{}) string {
	md, ok := obj["metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	ns, _ := md["namespace"].(string)
	return ns
}

// Names maps a decoded list to its objects' metadata.name values — the shape the
// api's live-dropdown proxies and several summaries want.
func Names(items []interface{}) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		if obj, ok := it.(map[string]interface{}); ok {
			if n := ObjectName(obj); n != "" {
				out = append(out, n)
			}
		}
	}
	return out
}

// Context returns the context every action runs its API calls under. The flow
// engine has no per-node deadline, so this bounds a pathological call (a hung
// API server behind a silently-dropping firewall) at a little over the client
// timeout rather than never.
func Context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout+15*time.Second)
}
