// Package awx holds the shared credential handling, API-root discovery, HTTP
// plumbing, pagination, launch pre-flight and job-waiting machinery used by
// every infrastructure/awx/* action.
//
// AWX (and its downstream product, Red Hat Ansible Automation Platform) exposes
// a uniform DRF REST API: every collection paginates the same way, every object
// is addressed by an integer id, and every write is JSON. That regularity lets
// the request, error translation, pagination and result shaping live here once
// so each action package stays thin: read its inputs, call one helper, shape the
// result.
//
// Five AWX-specific hazards are handled here so the 59 actions do not have to:
//
//   - API ROOT. Upstream AWX and AAP ≤ 2.4 serve the controller at /api/v2/;
//     AAP 2.5+ behind the platform gateway serves it at /api/controller/v2/.
//     ResolveAPIRoot discovers which, once per process per base URL, so the
//     operator never has to know. See its doc comment — it is the hardest thing
//     in this file and the rule about 401s not sweeping is load-bearing.
//
//   - TRAILING SLASHES. Django's APPEND_SLASH 301s a slash-less POST, and Go's
//     http.Client turns a redirected POST into a GET *and drops the body* —
//     producing a mystery empty-payload launch. Every path goes through
//     ensureTrailingSlash.
//
//   - SILENTLY IGNORED LAUNCH PROMPTS. AWX answers 201 and drops any prompt
//     field whose ask_*_on_launch flag is off, recording it only in the
//     response's ignored_fields. Sending limit=web* to a template that does not
//     prompt for it runs the playbook against EVERY host. ValidateLaunch
//     pre-flights and fails closed.
//
//   - ASYNCHRONOUS EVENTS. A job's status flips to successful the instant the
//     runner exits, but its stdout, events and artifacts may still be flushing
//     to Postgres. WaitForJob gates on event_processing_finished before reading
//     any of them.
//
//   - TYPE INSTABILITY. job.artifacts is either a JSON object or the literal
//     string "$hidden due to Ansible no_log flag$"; a launch 201 is a job unless
//     the template is sliced, in which case it is a workflow job with no `job`
//     key at all. DecodeArtifacts and LaunchedJob absorb both.
package awx

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// AuthMethodToken is a Bearer personal access token — the default and the
	// only method that works everywhere (SSO users included).
	AuthMethodToken = "token"
	// AuthMethodBasic is username+password. A fallback: AWX admins routinely set
	// AUTH_BASIC_ENABLED=false (Red Hat recommends it) and it never works for an
	// SSO-backed user.
	AuthMethodBasic = "basic"

	// Template kinds accepted by PreflightLaunch / FetchSurveySpec.
	TemplateKindJob      = "job_template"
	TemplateKindWorkflow = "workflow_job_template"

	// Job kinds — AWX's five "unified job" types.
	JobKindJob             = "job"
	JobKindWorkflowJob     = "workflow_job"
	JobKindAdHocCommand    = "ad_hoc_command"
	JobKindProjectUpdate   = "project_update"
	JobKindInventoryUpdate = "inventory_update"

	// maxResponseBody caps a response body to prevent memory exhaustion. Job
	// stdout can be large, so 8 MB rather than 1 MB.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single AWX call. Waiting
	// for a job is done by polling, not by holding a request open, so no single
	// call should ever need longer than this.
	requestTimeout = 60 * time.Second

	// MaxPageSize is AWX's MAX_PAGE_SIZE. A larger value is SILENTLY CLAMPED by
	// AWX, so we clamp it ourselves and never assume we got everything.
	MaxPageSize = 200
	// DefaultPageSize is the page size a *_list action uses when the operator
	// leaves Page Size blank.
	DefaultPageSize = 50
	// MaxAllPages bounds a Return All walk: 50 × 200 = 10,000 rows.
	MaxAllPages = 50

	// Waiting. DefaultWaitSeconds is what a waiting action uses when Timeout is
	// blank; MaxWaitSeconds is a hard cap, because a waiting node pins a flow
	// worker for its whole duration.
	DefaultWaitSeconds = 600
	MaxWaitSeconds     = 3600
	// DefaultPollSeconds is the floor for the poll interval. It is a floor, not
	// a fixed period: pollInterval stretches it on a long-running job.
	DefaultPollSeconds = 5
	// DefaultStdoutMaxBytes is the client-side stdout cap (1 MiB) when the
	// operator leaves Max Bytes blank.
	DefaultStdoutMaxBytes = 1 << 20

	// eventSettleTimeout bounds the extra wait for AWX to finish writing a job's
	// events after the job itself has gone terminal. Usually one extra poll.
	eventSettleTimeout  = 15 * time.Second
	eventSettleInterval = 2 * time.Second

	// stdoutTooLargeSentence is what AWX puts in a 200 response BODY when a job's
	// output exceeds STDOUT_MAX_BYTES_DISPLAY. A naive client stores this English
	// sentence as the playbook output; FetchStdout reports it as an error.
	stdoutTooLargeSentence = "Standard Output too large to display"
)

// ---------------------------------------------------------------------------
// Shared input declarations
// ---------------------------------------------------------------------------

// AuthInputs is the canonical credential block. Action packages re-declare their
// own literal Inputs arrays (the manifest generator AST-parses those literals, so
// it cannot see through a shared variable), but every action puts these seven
// first, in this order. awx_inputs_drift_test.go is what enforces it.
//
// The names matter. core.FindConnection returns the FIRST input whose name
// matches, and auth inputs are declared first — so an auth field that shares a
// name with a resource field silently shadows it, and the action reads the
// credential where it meant to read the resource. AWX is unusually exposed to
// this: url, token, username, password, name, organization, inventory,
// credential and description are all plausible on both sides. Hence awx_url (not
// url), api_token (not token), awx_username / awx_password (not username /
// password).
var AuthInputs = []core.Connection{
	{
		Name:        "awx_url",
		Type:        core.ConnectionTypeString,
		Label:       "AWX / AAP URL",
		Placeholder: "https://awx.example.com — your AWX or Ansible Automation Platform address",
		Required:    true,
	},
	{
		Name:  "auth_method",
		Type:  core.ConnectionTypeString,
		Label: "Authentication",
		Options: []core.ConnectionOption{
			{Name: "API Token (recommended)", Value: AuthMethodToken},
			{Name: "Username & Password", Value: AuthMethodBasic},
		},
	},
	{
		// NOT "token" — a resource field named token would resolve to this.
		Name:        "api_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Token",
		Placeholder: "AWX ▸ your user ▸ Tokens ▸ Add, Application blank, Scope = Write. Shown once — copy it then.",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"", AuthMethodToken}},
	},
	{
		// NOT "username" — AWX users have one too.
		Name:        "awx_username",
		Type:        core.ConnectionTypeString,
		Label:       "Username",
		Placeholder: "your AWX username",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthMethodBasic}},
	},
	{
		// NOT "password" — AWX users have one too.
		Name:        "awx_password",
		Type:        core.ConnectionTypeSecret,
		Label:       "Password",
		Placeholder: "your AWX password — note some AWX installs disable password authentication",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthMethodBasic}},
	},
	{
		Name:        "allow_insecure",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Allow Insecure TLS",
		Placeholder: "Skip certificate verification — only for a self-hosted AWX with a self-signed certificate",
	},
	{
		Name:        "api_prefix",
		Type:        core.ConnectionTypeString,
		Label:       "API Path Prefix (advanced)",
		Placeholder: "Leave blank — detected automatically. Only set this if support asks (e.g. /api/controller/v2/).",
	},
}

// ConfirmDestructiveInput is the guard every destructive action carries, LAST and
// Required. It is a boolean so the editor renders a checkbox, but the editor also
// offers a variable picker on booleans — so a flow can bind it to ${var.approved}
// or to an upstream node's boolean output and gate the action on a condition
// evaluated at run time, rather than on a box someone ticked at design time.
//
// See ConfirmDestructive for why the value is read without Connection.Boolean().
var ConfirmDestructiveInput = core.Connection{
	Name:        "confirm_destructive",
	Type:        core.ConnectionTypeBoolean,
	Label:       "Confirm Destructive Action",
	Placeholder: "This permanently changes AWX state. Tick to allow, or bind a variable such as ${var.approved}",
	Required:    true,
}

// ---------------------------------------------------------------------------
// Credentials
// ---------------------------------------------------------------------------

// Auth is a resolved AWX credential for one action invocation.
type Auth struct {
	// BaseURL is normalised: scheme://host[/context-path], no trailing slash and
	// no /api… suffix.
	BaseURL string
	// Method is AuthMethodToken or AuthMethodBasic.
	Method    string
	Token     string
	Username  string
	Password  string
	Insecure  bool
	APIPrefix string // operator override; "" means auto-detect
}

// GetAuth resolves and validates the seven credential inputs.
//
// A failure here is the ONE case an action returns as a HARD error (nil outputs,
// non-nil error): the node is mis-configured, not the request. Everything else —
// including every AWX 4xx — is a SOFT failure via ErrorResult.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	base, err := NormaliseBaseURL(OptionalString("awx_url", inputs))
	if err != nil {
		return Auth{}, err
	}

	a := Auth{
		BaseURL:   base,
		Method:    OptionalString("auth_method", inputs),
		Token:     OptionalString("api_token", inputs),
		Username:  OptionalString("awx_username", inputs),
		Password:  OptionalString("awx_password", inputs),
		Insecure:  BoolInput("allow_insecure", inputs),
		APIPrefix: OptionalString("api_prefix", inputs),
	}

	// A blank auth_method means the operator never touched the dropdown, which
	// is why the token field is visible when auth_method is "" as well as
	// "token". Bearer is the default because Basic can be disabled server-side.
	if a.Method == "" {
		a.Method = AuthMethodToken
	}

	switch a.Method {
	case AuthMethodToken:
		if a.Token == "" {
			return Auth{}, errors.New("API Token is required — create one in AWX under your user ▸ Tokens (Scope: Write), or switch Authentication to Username & Password")
		}
	case AuthMethodBasic:
		if a.Username == "" || a.Password == "" {
			return Auth{}, errors.New("Username and Password are both required when Authentication is Username & Password")
		}
	default:
		return Auth{}, fmt.Errorf("unknown Authentication method %q — choose API Token or Username & Password", a.Method)
	}
	return a, nil
}

// applyAuth sets the Authorization header. AWX also accepts the legacy
// "Token <pat>" scheme, but RFC 6750 Bearer is what the OAuth2 provider
// documents and what AAP's gateway proxies — do not rely on the legacy form.
func (a Auth) applyAuth(req *http.Request) {
	if a.Method == AuthMethodBasic {
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(a.Username+":"+a.Password)))
		return
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
}

// NormaliseBaseURL trims and validates a pasted controller URL into a bare base
// with no trailing slash. A bare host is tolerated (https is assumed); a full
// http(s) URL — including one served under a context path — is preserved.
// Query/fragment are stripped so a crafted value cannot smuggle a query string
// onto every request, and userinfo is dropped.
//
// It also strips a pasted API suffix. Operators paste the URL out of their
// browser's address bar or out of the API docs constantly, so
// https://awx/api/v2/ has to mean the same thing as https://awx.
func NormaliseBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("AWX / AAP URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", errors.New("AWX / AAP URL must be a full http(s) URL, e.g. https://awx.example.com")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", errors.New("AWX / AAP URL must start with http:// or https://")
	}

	u.User = nil // drop any user:pass@ smuggled into the pasted URL

	p := strings.TrimRight(u.Path, "/")
	for _, suffix := range []string{"/api/controller/v2", "/api/controller", "/api/v2", "/api"} {
		if strings.HasSuffix(p, suffix) {
			p = strings.TrimSuffix(p, suffix)
			break
		}
	}
	u.Path = p
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

// ensureTrailingSlash appends the slash Django's APPEND_SLASH demands, splitting
// any query string off first so "jobs?id=4" becomes "jobs/?id=4" rather than
// "jobs?id=4/".
//
// This is not cosmetic. A POST to a slash-less path is answered with a 301, and
// Go's http.Client follows a 301 by re-issuing the request AS A GET WITH NO BODY
// — so the launch silently becomes a read and nothing runs.
func ensureTrailingSlash(path string) string {
	p, q := path, ""
	if i := strings.IndexByte(path, '?'); i >= 0 {
		p, q = path[:i], path[i:]
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p + q
}

// ---------------------------------------------------------------------------
// ★ API-root discovery
// ---------------------------------------------------------------------------

// CandidateRoots are the only two roots any AWX or AAP has ever served, in
// most-likely-first order.
//
//	upstream AWX (any version)          /api/v2/
//	AAP ≤ 2.4                           /api/v2/
//	AAP 2.5+ behind the platform gateway  /api/controller/v2/   (the legacy
//	                                    /api/v2/ is still proxied, but is
//	                                    deprecated and slated for removal)
var CandidateRoots = []string{"/api/v2/", "/api/controller/v2/"}

// APIRoot is a discovered controller API prefix plus the product banner AWX
// returns alongside it. Prefix always carries a leading AND a trailing slash.
type APIRoot struct {
	Prefix  string // e.g. "/api/v2/"
	Product string // X-API-Product-Name, e.g. "AWX" or "Red Hat Ansible Automation Platform"
	Version string // X-API-Product-Version, e.g. "24.6.1"
}

var (
	rootCacheMu sync.RWMutex
	// rootCache is keyed on Auth.BaseURL ONLY. The API root is a property of the
	// SERVER, not of the credential, so two flows with different tokens against
	// the same controller share the discovery. Successes only are cached — a
	// transient network blip must not poison the process — and there is no TTL:
	// an AWX does not move its API root at run time, and the one-shot executor
	// process is short-lived anyway.
	rootCache = map[string]APIRoot{}
)

// ResetAPIRootCacheForTest clears the discovery cache. Tests that stand up a new
// httptest server must call it, or the previous server's root leaks in.
func ResetAPIRootCacheForTest() {
	rootCacheMu.Lock()
	rootCache = map[string]APIRoot{}
	rootCacheMu.Unlock()
}

// credentialError marks an authentication failure, which is the one outcome of
// the root probe that must NOT make us try the other candidate root.
type credentialError struct{ msg string }

func (e *credentialError) Error() string { return e.msg }

// ResolveAPIRoot returns the controller API prefix for this instance,
// discovering it once per (process, base URL) and caching it. Two round-trips on
// a cold cache, zero thereafter.
//
//  1. An operator override (API Path Prefix) wins outright — no probe at all.
//  2. Cache.
//  3. Unauthenticated GET {base}/api/ :
//     available_versions.v2 present -> that value is the prefix (normally /api/v2/)
//     current_version present       -> use that
//     neither                       -> this is the AAP 2.5 gateway, whose root
//     lacks available_versions entirely (proven by
//     awx#16054, where `awx login` gets a 200 from the
//     gateway and then crashes on the missing attribute)
//     -> try /api/controller/v2/ first
//     A 401 here means the gateway gates even /api/ — retried ONCE with
//     credentials. Any other failure is non-fatal: we fall through to the sweep,
//     which is authoritative.
//  4. Confirm with an AUTHENTICATED GET {prefix}me/ :
//     200     -> cache and return (capturing the X-API-Product-* headers)
//     401/403 -> ★ THE PREFIX IS RIGHT AND THE CREDENTIAL IS WRONG. Return the
//     credential error immediately. Do NOT sweep the other candidate:
//     sweeping would turn "your token is invalid" into "we cannot find
//     AWX", which is the single most misleading failure this node could
//     produce.
//     other   -> wrong prefix; try the next candidate.
//  5. All candidates exhausted -> a hard, actionable message naming both roots
//     and pointing at the API Path Prefix escape hatch.
//
// me/ is the confirm probe rather than ping/ deliberately: on bare AWX, ping/ is
// AllowAny with authentication_classes = (), so it answers 200 for a garbage
// token and tells us nothing about the credential. me/ requires auth on both
// deployments, which is exactly what separates "wrong prefix" (404) from "wrong
// credential" (401).
func ResolveAPIRoot(ctx context.Context, a Auth) (APIRoot, error) {
	if override := strings.TrimSpace(a.APIPrefix); override != "" {
		return APIRoot{Prefix: normalisePrefix(override)}, nil
	}

	rootCacheMu.RLock()
	cached, ok := rootCache[a.BaseURL]
	rootCacheMu.RUnlock()
	if ok {
		return cached, nil
	}

	for _, prefix := range probeCandidates(ctx, a) {
		root, err := confirmRoot(ctx, a, prefix)
		if err != nil {
			var credErr *credentialError
			if errors.As(err, &credErr) {
				// The prefix is right; the credential is not. Never sweep.
				return APIRoot{}, err
			}
			continue // wrong prefix (404) or unreachable — try the next one
		}
		rootCacheMu.Lock()
		rootCache[a.BaseURL] = root
		rootCacheMu.Unlock()
		return root, nil
	}

	return APIRoot{}, fmt.Errorf(
		"Could not find the AWX / AAP API at %s. Checked %s and %s. "+
			"Check the AWX / AAP URL is right and reachable, or — if your platform serves the controller API somewhere else — set the API Path Prefix on this node.",
		Redact(a, a.BaseURL), CandidateRoots[0], CandidateRoots[1])
}

// probeCandidates orders the candidate roots using the unauthenticated /api/
// banner, falling back to CandidateRoots unchanged when the probe tells us
// nothing. The probe is an optimisation and a tie-breaker; the authenticated
// sweep in ResolveAPIRoot is what is authoritative.
func probeCandidates(ctx context.Context, a Auth) []string {
	body, ok := fetchAPIBanner(ctx, a)
	if !ok {
		return CandidateRoots
	}

	prefix := ""
	if versions, ok := body["available_versions"].(map[string]interface{}); ok {
		if v, ok := versions["v2"].(string); ok && v != "" {
			prefix = v
		}
	}
	if prefix == "" {
		if v, ok := body["current_version"].(string); ok && v != "" {
			prefix = v
		}
	}
	if prefix == "" {
		// No available_versions and no current_version: the AAP 2.5 platform
		// gateway. We key on the ABSENCE of a field we know upstream AWX has,
		// never on a gateway field name we would be guessing at — the gateway is
		// closed source and its root body is undocumented.
		prefix = "/api/controller/v2/"
	}

	prefix = normalisePrefix(prefix)
	ordered := []string{prefix}
	for _, c := range CandidateRoots {
		if c != prefix {
			ordered = append(ordered, c)
		}
	}
	return ordered
}

// fetchAPIBanner GETs {base}/api/ unauthenticated, retrying once with the
// credential if the deployment gates even that. Any failure is reported as
// !ok rather than as an error: the caller falls back to the sweep.
func fetchAPIBanner(ctx context.Context, a Auth) (map[string]interface{}, bool) {
	resp, err := request(ctx, a, http.MethodGet, a.BaseURL+"/api/", nil, "application/json", false)
	if err != nil {
		return nil, false
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		resp, err = request(ctx, a, http.MethodGet, a.BaseURL+"/api/", nil, "application/json", true)
		if err != nil {
			return nil, false
		}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, false
	}
	body := map[string]interface{}{}
	if json.Unmarshal(resp.Body, &body) != nil {
		return nil, false
	}
	return body, true
}

// confirmRoot performs the authenticated GET {prefix}me/ that decides whether a
// candidate prefix is the real controller root.
func confirmRoot(ctx context.Context, a Auth, prefix string) (APIRoot, error) {
	resp, err := doRaw(ctx, a, http.MethodGet, a.BaseURL+prefix+"me/", nil)
	if err != nil {
		return APIRoot{}, err
	}
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return APIRoot{
			Prefix:  prefix,
			Product: resp.Header.Get("X-API-Product-Name"),
			Version: resp.Header.Get("X-API-Product-Version"),
		}, nil
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		return APIRoot{}, &credentialError{msg: Redact(a, credentialRejected(a, resp.StatusCode))}
	default:
		return APIRoot{}, fmt.Errorf("AWX API not found at %s (HTTP %d)", prefix, resp.StatusCode)
	}
}

// normalisePrefix forces a leading and a trailing slash onto an API prefix, so
// "api/v2", "/api/v2" and "/api/v2/" all mean the same thing.
func normalisePrefix(p string) string {
	p = strings.TrimSpace(p)
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	if !strings.HasSuffix(p, "/") {
		p += "/"
	}
	return p
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// httpClient is shared across every AWX action so TCP connections to a
// controller are pooled and reused rather than re-dialled per call.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// insecureHTTPClient skips certificate verification. It is a separate,
// opt-in client so the secure default above can never be weakened by a
// per-request tweak.
var insecureHTTPClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		// #nosec G402 -- opt-in only, via the Allow Insecure TLS input, for a
		// self-hosted AWX with a self-signed certificate.
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: true},
	},
}

func clientFor(a Auth) *http.Client {
	if a.Insecure {
		return insecureHTTPClient
	}
	return httpClient
}

// Context returns the context an action runs its API calls under. core.Action
// takes no context — the flow engine has no per-node deadline — so this bounds a
// pathological call (a hung controller behind a silently-dropping firewall) at a
// little over the client timeout rather than never.
//
// A WAITING action can use this too: WaitForJob deliberately does not inherit its
// caller's deadline (see its doc comment), so a 10-minute wait is not cut short by
// the 75 seconds granted here.
func Context() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), requestTimeout+15*time.Second)
}

// Response is the raw outcome of an AWX call. Method and URL are retained so
// CheckResponse can translate a status that means different things on different
// endpoints (a 403 on a launch is a scope/role problem, not a bad token).
// Truncated is set when the body hit maxResponseBody, so a caller reporting job
// output can say the data is incomplete rather than reporting a clean success on
// a silently-clipped log.
type Response struct {
	StatusCode int
	Body       []byte
	Header     http.Header
	Location   string
	Truncated  bool
	Method     string
	URL        string
}

// Do performs one AWX API call. path is API-ROOT-RELATIVE ("job_templates/7/",
// "jobs/?id=4") — Do resolves the root, forces the trailing slash, and JSON-
// encodes body (pass nil for no body).
//
// It returns an error only for a transport-level failure; an AWX 4xx/5xx comes
// back as a Response for CheckResponse to translate.
func Do(ctx context.Context, a Auth, method, path string, body interface{}) (*Response, error) {
	root, err := ResolveAPIRoot(ctx, a)
	if err != nil {
		return nil, err
	}
	rel := ensureTrailingSlash(strings.TrimPrefix(path, "/"))
	return doRaw(ctx, a, method, a.BaseURL+root.Prefix+rel, body)
}

// doRaw performs one call against an already-absolute URL. It is what the
// root-discovery probe and the pagination walker use, since neither can go
// through Do (one runs before the root is known, the other is handed a
// fully-formed next link).
func doRaw(ctx context.Context, a Auth, method, rawURL string, body interface{}) (*Response, error) {
	return request(ctx, a, method, rawURL, body, "application/json", true)
}

func request(ctx context.Context, a Auth, method, rawURL string, body interface{}, accept string, withAuth bool) (*Response, error) {
	payload, err := encodeBody(body)
	if err != nil {
		return nil, err
	}

	var reader io.Reader
	if payload != nil {
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", Redact(a, err.Error()))
	}
	if withAuth {
		a.applyAuth(req)
	}
	req.Header.Set("Accept", accept)
	if payload != nil {
		// AWX registers ONLY JSONParser. A form-encoded body is a flat 415.
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := clientFor(a).Do(req)
	if err != nil {
		return nil, fmt.Errorf("AWX request failed: %s", Redact(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	// Read one byte past the cap so an exactly-at-cap body is distinguishable
	// from a clipped one.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read the AWX response: %s", Redact(a, err.Error()))
	}
	truncated := false
	if len(raw) > maxResponseBody {
		raw = raw[:maxResponseBody]
		truncated = true
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       raw,
		Header:     resp.Header,
		Location:   resp.Header.Get("Location"),
		Truncated:  truncated,
		Method:     method,
		URL:        rawURL,
	}, nil
}

// encodeBody JSON-encodes a request body, returning nil for "no body". A typed
// nil map (var m map[string]interface{}) would otherwise marshal to the literal
// null, which AWX rejects.
func encodeBody(body interface{}) ([]byte, error) {
	switch b := body.(type) {
	case nil:
		return nil, nil
	case map[string]interface{}:
		if b == nil {
			return nil, nil
		}
	case []byte:
		return b, nil
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the request body: %w", err)
	}
	return payload, nil
}

// ---------------------------------------------------------------------------
// Error translation
// ---------------------------------------------------------------------------

// CheckResponse turns an AWX status into an operator-readable error, or nil when
// the call succeeded. Codes listed in acceptable are treated as success (a cancel
// answers 202, a delete 204, an inventory delete 202).
//
// Four statuses are translated bespokely, because they are where operators lose
// hours:
//
//   - 401 — the token is wrong/expired/revoked. AWX shows a token exactly once.
//   - 403 on a POST to …/launch/ — a READ-scoped token authenticates fine on
//     every GET and only fails here, so without this message every operator
//     concludes the token is broken when it is merely read-only.
//   - 404 — AWX hides objects you cannot SEE behind a 404 rather than a 403, so
//     this must never be reported as "deleted".
//   - 409 with active_jobs — retryable: something is still running against the
//     object, not a permanent failure.
func CheckResponse(a Auth, resp *Response, acceptable ...int) error {
	if resp == nil {
		return errors.New("AWX returned no response")
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	for _, code := range acceptable {
		if resp.StatusCode == code {
			return nil
		}
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return errors.New(Redact(a, credentialRejected(a, resp.StatusCode)))

	case http.StatusForbidden:
		if isLaunchRequest(resp) {
			return errors.New("AWX accepted the credential but refused the launch (HTTP 403). Either the API token is READ-scoped — create a new one with Scope = Write — or this user does not have the Execute role on that job template.")
		}
		if detail := decodeErrorBody(resp.Body); detail != "" {
			return fmt.Errorf("AWX refused the request (HTTP 403): %s", Redact(a, detail))
		}
		return errors.New("AWX refused the request (HTTP 403) — this AWX user does not have permission to do that.")

	case http.StatusNotFound:
		msg := "AWX returned 404. The object may not exist — or your user may not have permission to see it: AWX hides objects you cannot view behind a 404 rather than a 403."
		if detail := decodeErrorBody(resp.Body); detail != "" && !strings.EqualFold(detail, "not found.") {
			msg += " AWX said: " + Redact(a, detail)
		}
		return errors.New(msg)

	case http.StatusConflict:
		if msg, ok := activeJobsMessage(resp.Body); ok {
			return errors.New(Redact(a, msg))
		}
	}

	if msg := decodeErrorBody(resp.Body); msg != "" {
		return fmt.Errorf("AWX API error (%d): %s", resp.StatusCode, Redact(a, msg))
	}
	return fmt.Errorf("AWX API error (%d)", resp.StatusCode)
}

// credentialRejected is the 401 message, shared by CheckResponse and the API-root
// confirm probe so the operator sees the same words wherever the token fails.
func credentialRejected(a Auth, status int) string {
	if a.Method == AuthMethodBasic {
		return fmt.Sprintf("AWX rejected the credential (HTTP %d). Check the Username and Password — or this AWX has password authentication disabled (AUTH_BASIC_ENABLED=false), which is common, and for an SSO user it never works. Use an API token instead.", status)
	}
	return fmt.Sprintf("AWX rejected the credential (HTTP %d). The API token may be wrong, expired or revoked. Tokens are shown only once when they are created — if you have lost it, create a new one in AWX under your user ▸ Tokens.", status)
}

// isLaunchRequest reports whether a response came from POSTing a launch, which is
// the only place a read-scoped token fails.
func isLaunchRequest(resp *Response) bool {
	return resp.Method == http.MethodPost && strings.Contains(resp.URL, "/launch/")
}

// activeJobsMessage translates AWX's 409 envelope:
//
//	{"error": "Resource is being used by running jobs.",
//	 "active_jobs": [{"type": "job", "id": 12}]}
func activeJobsMessage(body []byte) (string, bool) {
	var envelope struct {
		Error      string `json:"error"`
		ActiveJobs []struct {
			Type string      `json:"type"`
			ID   interface{} `json:"id"`
		} `json:"active_jobs"`
	}
	if json.Unmarshal(body, &envelope) != nil || len(envelope.ActiveJobs) == 0 {
		return "", false
	}
	refs := make([]string, 0, len(envelope.ActiveJobs))
	for _, j := range envelope.ActiveJobs {
		kind := j.Type
		if kind == "" {
			kind = "job"
		}
		refs = append(refs, strings.ReplaceAll(kind, "_", " ")+" "+IDString(j.ID))
	}
	return fmt.Sprintf("AWX refused: a job is still running against this resource (%s). Wait for it to finish, or cancel it, then try again.",
		strings.Join(refs, ", ")), true
}

// decodeErrorBody reduces one of AWX's error envelopes to a single line:
//
//	{"detail": "..."}                    -> the detail
//	{"error": "..."}                     -> the error
//	{"__all__": ["..."]}                 -> the non-field error
//	{"field": ["msg", ...], ...}         -> "field: msg; msg" with stable key order
//
// A non-JSON body (an HTML error page from a proxy in front of AWX) is reduced to
// a short tag-stripped snippet.
func decodeErrorBody(body []byte) string {
	if len(bytes.TrimSpace(body)) == 0 {
		return ""
	}

	var envelope map[string]interface{}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return snippet(body)
	}

	for _, key := range []string{"detail", "error", "__all__"} {
		if v, ok := envelope[key]; ok {
			if s := flattenErrorValue(v); s != "" {
				return s
			}
		}
	}

	keys := make([]string, 0, len(envelope))
	for k := range envelope {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		if s := flattenErrorValue(envelope[k]); s != "" {
			parts = append(parts, k+": "+s)
		}
	}
	if len(parts) == 0 {
		return snippet(body)
	}
	return strings.Join(parts, "; ")
}

// flattenErrorValue renders one field's error value — a string, or the list of
// strings DRF usually produces — as a single line.
func flattenErrorValue(v interface{}) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case []interface{}:
		parts := make([]string, 0, len(t))
		for _, item := range t {
			if s := flattenErrorValue(item); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "; ")
	case map[string]interface{}:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(keys))
		for _, k := range keys {
			if s := flattenErrorValue(t[k]); s != "" {
				parts = append(parts, k+": "+s)
			}
		}
		return strings.Join(parts, "; ")
	case nil:
		return ""
	default:
		return fmt.Sprintf("%v", t)
	}
}

// snippet reduces a non-JSON error body — usually an HTML page from a proxy or
// load balancer in front of AWX — to a short, single-line hint.
func snippet(body []byte) string {
	s := string(body)
	if i := strings.Index(s, "<body"); i >= 0 {
		s = s[i:]
	}
	s = stripTags(s)
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}

func stripTags(s string) string {
	var b strings.Builder
	depth := 0
	for _, r := range s {
		switch r {
		case '<':
			depth++
		case '>':
			if depth > 0 {
				depth--
			}
		default:
			if depth == 0 {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// Redact scrubs credential material out of a string bound for an error message or
// a log line. A bearer token travels in a header rather than a URL, so a leak is
// unlikely — but a proxy error or a wrapped transport error can echo one, and an
// error in a flow's output is an error in the audit log.
//
// It is a plain substring replace, with no minimum secret length. That means a
// degenerate credential (a password of "job", say) would also scrub those letters
// out of otherwise innocent words. That over-redaction is deliberate: a mangled
// error message is cosmetic, an under-redacted credential is a security incident.
//
// Applied at every error-construction site in this package.
func Redact(a Auth, msg string) string {
	for _, secret := range []string{a.Token, a.Password} {
		if secret != "" {
			msg = strings.ReplaceAll(msg, secret, "REDACTED")
		}
	}
	if a.Method == AuthMethodBasic && a.Username != "" && a.Password != "" {
		blob := base64.StdEncoding.EncodeToString([]byte(a.Username + ":" + a.Password))
		msg = strings.ReplaceAll(msg, blob, "REDACTED")
	}
	return msg
}

// ---------------------------------------------------------------------------
// Decoding + generic CRUD
// ---------------------------------------------------------------------------

// DecodeObject unmarshals a JSON object response into a map. An empty body — an
// attach (204) or a cancel (202) — yields an empty map rather than an error.
func DecodeObject(resp *Response) (map[string]interface{}, error) {
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return map[string]interface{}{}, nil
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse the AWX response: %w", err)
	}
	return out, nil
}

// GetResource GETs one object. path is API-root-relative; q may be nil.
func GetResource(ctx context.Context, a Auth, path string, q url.Values) (map[string]interface{}, error) {
	if q != nil {
		if enc := q.Encode(); enc != "" {
			path = ensureTrailingSlash(path) + "?" + enc
		}
	}
	resp, err := Do(ctx, a, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(a, resp); err != nil {
		return nil, err
	}
	return DecodeObject(resp)
}

// CreateResource POSTs a body. It accepts AWX's 200/201/202 — a project sync and
// an inventory delete both answer 202, and an attach answers 204 with an empty
// body, which decodes to an empty map.
func CreateResource(ctx context.Context, a Auth, path string, body map[string]interface{}) (map[string]interface{}, error) {
	resp, err := Do(ctx, a, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(a, resp, http.StatusAccepted, http.StatusNoContent); err != nil {
		return nil, err
	}
	return DecodeObject(resp)
}

// UpdateResource PATCHes a body.
//
// ★ PATCH, NEVER PUT — this holds for every update action in the node. AWX copies
// each model field's default onto the serializer field, so a PUT that omits a
// field RESETS it to the model default. PUT is a genuine destructive full-replace.
func UpdateResource(ctx context.Context, a Auth, path string, body map[string]interface{}) (map[string]interface{}, error) {
	resp, err := Do(ctx, a, http.MethodPatch, path, body)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(a, resp); err != nil {
		return nil, err
	}
	return DecodeObject(resp)
}

// DeleteResource DELETEs an object, returning the status code so a caller can tell
// AWX's asynchronous 202 (an inventory delete is queued, and the row lingers with
// pending_deletion=true) from a synchronous 204.
func DeleteResource(ctx context.Context, a Auth, path string) (int, error) {
	resp, err := Do(ctx, a, http.MethodDelete, path, nil)
	if err != nil {
		return 0, err
	}
	if err := CheckResponse(a, resp, http.StatusAccepted, http.StatusNoContent); err != nil {
		return resp.StatusCode, err
	}
	return resp.StatusCode, nil
}

// FetchMe returns the AWX user the credential belongs to. /me/ is a PAGINATED
// LIST, not an object — this unwraps results[0].
func FetchMe(ctx context.Context, a Auth) (map[string]interface{}, error) {
	obj, err := GetResource(ctx, a, "me/", nil)
	if err != nil {
		return nil, err
	}
	results, _ := obj["results"].([]interface{})
	if len(results) == 0 {
		return nil, errors.New("AWX did not report a current user for this credential")
	}
	me, ok := results[0].(map[string]interface{})
	if !ok {
		return nil, errors.New("AWX returned an unexpected shape for the current user")
	}
	return me, nil
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

// List fetches a collection. AWX's envelope is {count, next, previous, results}.
//
// count is the TOTAL number of matching rows on the server, not the number
// returned. hasMore reports that rows remain — either because only one page was
// asked for, or because a Return All walk hit MaxAllPages.
//
// Two AWX quirks are handled here:
//
//   - page_size is clamped to MaxPageSize. AWX clamps it silently anyway, so a
//     caller that trusted its own page_size would quietly miss rows.
//   - next/previous are RELATIVE PATHS, not absolute URLs (AWX overrides DRF's
//     get_next_link to use request.get_full_path()), so they are resolved against
//     the base before being followed.
//
// Never send ?limit= to an events endpoint: UnifiedJobEventPagination silently
// switches to LimitPagination and the response loses count/next/previous
// entirely. A page-size input must only ever map to page_size.
func List(ctx context.Context, a Auth, path string, q url.Values, returnAll bool) (items []interface{}, count int, hasMore bool, err error) {
	root, err := ResolveAPIRoot(ctx, a)
	if err != nil {
		return nil, 0, false, err
	}

	if q == nil {
		q = url.Values{}
	}
	if raw := q.Get("page_size"); raw != "" {
		n, convErr := strconv.Atoi(raw)
		q.Set("page_size", strconv.Itoa(ClampPageSize(n, convErr == nil)))
	}

	next := a.BaseURL + root.Prefix + ensureTrailingSlash(strings.TrimPrefix(path, "/"))
	if enc := q.Encode(); enc != "" {
		next += "?" + enc
	}

	items = []interface{}{}
	pages := 0
	for next != "" {
		resp, err := doRaw(ctx, a, http.MethodGet, next, nil)
		if err != nil {
			return nil, 0, false, err
		}
		if err := CheckResponse(a, resp); err != nil {
			return nil, 0, false, err
		}

		var page struct {
			Count   int           `json:"count"`
			Next    *string       `json:"next"`
			Results []interface{} `json:"results"`
		}
		if err := json.Unmarshal(resp.Body, &page); err != nil {
			return nil, 0, false, fmt.Errorf("failed to parse the AWX list response: %w", err)
		}

		pages++
		if pages == 1 {
			count = page.Count
		}
		items = append(items, page.Results...)

		more := page.Next != nil && strings.TrimSpace(*page.Next) != ""
		if !returnAll || !more {
			return items, count, more, nil
		}
		if pages >= MaxAllPages {
			// Bounded deliberately: a Return All over a huge collection must not
			// spin unbounded and pin the flow worker.
			return items, count, true, nil
		}

		next, err = resolveNext(a.BaseURL, *page.Next)
		if err != nil {
			return nil, 0, false, err
		}
	}
	return items, count, false, nil
}

// resolveNext turns AWX's relative next link into an absolute URL against the
// controller's own origin. An absolute link is only followed when it points at
// the same host — otherwise a compromised or misconfigured AWX could walk our
// bearer token to a server of its choosing.
func resolveNext(base, next string) (string, error) {
	n, err := url.Parse(strings.TrimSpace(next))
	if err != nil {
		return "", fmt.Errorf("AWX returned an unusable next-page link: %w", err)
	}
	b, err := url.Parse(base)
	if err != nil {
		return "", fmt.Errorf("AWX URL is unusable: %w", err)
	}
	if n.IsAbs() {
		if !strings.EqualFold(n.Host, b.Host) {
			return "", fmt.Errorf("AWX returned a next-page link pointing at a different host (%s) — refusing to follow it", n.Host)
		}
		return n.String(), nil
	}
	return b.ResolveReference(n).String(), nil
}

// ---------------------------------------------------------------------------
// ★ Launch pre-flight
// ---------------------------------------------------------------------------

// askFieldMap maps AWX's 16 ask_*_on_launch flags to the body field each one
// gates. Three do NOT follow the ask_X_on_launch → X rule, which is why this is
// hardcoded rather than derived by stripping the prefix and suffix:
//
//	ask_variables_on_launch  -> extra_vars
//	ask_tags_on_launch       -> job_tags
//	ask_credential_on_launch -> credentials
var askFieldMap = map[string]string{
	"ask_credential_on_launch":            "credentials",
	"ask_diff_mode_on_launch":             "diff_mode",
	"ask_execution_environment_on_launch": "execution_environment",
	"ask_forks_on_launch":                 "forks",
	"ask_instance_groups_on_launch":       "instance_groups",
	"ask_inventory_on_launch":             "inventory",
	"ask_job_slice_count_on_launch":       "job_slice_count",
	"ask_job_type_on_launch":              "job_type",
	"ask_labels_on_launch":                "labels",
	"ask_limit_on_launch":                 "limit",
	"ask_scm_branch_on_launch":            "scm_branch",
	"ask_skip_tags_on_launch":             "skip_tags",
	"ask_tags_on_launch":                  "job_tags",
	"ask_timeout_on_launch":               "timeout",
	"ask_variables_on_launch":             "extra_vars",
	"ask_verbosity_on_launch":             "verbosity",
}

// promptFields is the set of body fields that are gated by an ask_* flag —
// i.e. the fields AWX will silently drop if the template does not prompt for
// them. Anything not in here (credential_passwords, additional_fields spillover)
// is left for AWX to validate.
var promptFields = func() map[string]bool {
	out := map[string]bool{}
	for _, field := range askFieldMap {
		out[field] = true
	}
	return out
}()

// promptLabels give the operator the name AWX's own UI uses for a prompt field,
// so the refusal message names the checkbox they need to tick.
var promptLabels = map[string]string{
	"credentials":           "Credentials",
	"diff_mode":             "Show Changes (diff mode)",
	"execution_environment": "Execution Environment",
	"extra_vars":            "Variables",
	"forks":                 "Forks",
	"instance_groups":       "Instance Groups",
	"inventory":             "Inventory",
	"job_slice_count":       "Job Slicing",
	"job_tags":              "Job Tags",
	"job_type":              "Job Type",
	"labels":                "Labels",
	"limit":                 "Limit",
	"scm_branch":            "Source Control Branch",
	"skip_tags":             "Skip Tags",
	"timeout":               "Timeout",
	"verbosity":             "Verbosity",
}

// LaunchConfig is a template's answer to "what will you let me set at launch?" —
// the body of GET {job_templates|workflow_job_templates}/{id}/launch/.
type LaunchConfig struct {
	// Ask is keyed by the BODY FIELD name (limit, extra_vars, job_tags…), not by
	// the ask_* flag name.
	Ask map[string]bool
	// PasswordsNeededToStart lists the credential_passwords keys the template's
	// credentials will demand (ssh_password, vault_password.dev…).
	PasswordsNeededToStart []string
	// VariablesNeededToStart lists the REQUIRED SURVEY VARIABLE NAMES. Careful:
	// AWX reuses this key in a 400 body for human-readable ERROR STRINGS. Two
	// different meanings, two different code paths — never parse them with one.
	VariablesNeededToStart []string
	SurveyEnabled          bool
	InventoryNeededToStart bool
	// CanStartWithoutUserInput is false merely because prompting is AVAILABLE,
	// not because input is MANDATORY. Never treat it as "launch will fail".
	CanStartWithoutUserInput bool
	Defaults                 map[string]interface{}
	// Raw is the whole pre-flight body, for the action that exposes it.
	Raw map[string]interface{}
}

// PromptableFields lists, sorted, the body fields this template will accept at
// launch — i.e. the ask_* flags that are on, expressed as field names.
func (c LaunchConfig) PromptableFields() []string {
	out := make([]string, 0, len(c.Ask))
	for field, on := range c.Ask {
		if on {
			out = append(out, field)
		}
	}
	sort.Strings(out)
	return out
}

// PreflightLaunch fetches a template's launch configuration. kind is
// TemplateKindJob or TemplateKindWorkflow.
func PreflightLaunch(ctx context.Context, a Auth, kind string, id int64) (LaunchConfig, error) {
	path, err := TemplateKindPath(kind)
	if err != nil {
		return LaunchConfig{}, err
	}
	raw, err := GetResource(ctx, a, fmt.Sprintf("%s%d/launch/", path, id), nil)
	if err != nil {
		return LaunchConfig{}, err
	}

	cfg := LaunchConfig{
		Ask:                      map[string]bool{},
		Defaults:                 map[string]interface{}{},
		Raw:                      raw,
		SurveyEnabled:            BoolField(raw, "survey_enabled"),
		InventoryNeededToStart:   BoolField(raw, "inventory_needed_to_start"),
		CanStartWithoutUserInput: BoolField(raw, "can_start_without_user_input"),
	}
	for flag, field := range askFieldMap {
		cfg.Ask[field] = BoolField(raw, flag)
	}
	cfg.PasswordsNeededToStart = stringList(raw["passwords_needed_to_start"])
	cfg.VariablesNeededToStart = stringList(raw["variables_needed_to_start"])
	if defaults, ok := raw["defaults"].(map[string]interface{}); ok {
		cfg.Defaults = defaults
	}
	return cfg, nil
}

// SurveyQuestion is one question of a template's survey, with AWX's shapes
// normalised: choices arrive as a NEWLINE-SEPARATED STRING on some versions and
// as an array on others, and are always presented here as an array.
type SurveyQuestion struct {
	Variable            string      `json:"variable"`
	QuestionName        string      `json:"question_name"`
	QuestionDescription string      `json:"question_description,omitempty"`
	Type                string      `json:"type"` // text|textarea|password|multiplechoice|multiselect|integer|float
	Required            bool        `json:"required"`
	Default             interface{} `json:"default,omitempty"`
	Choices             []string    `json:"choices,omitempty"`
	Min                 *float64    `json:"min,omitempty"`
	Max                 *float64    `json:"max,omitempty"`
}

// SurveySpec is a template's survey. A template with no survey answers the
// survey_spec endpoint with HTTP 200 and an EMPTY OBJECT — not a 404 — so
// HasSurvey, not an error, is what tells you there is nothing to answer.
type SurveySpec struct {
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Spec        []SurveyQuestion `json:"spec"`
	// Raw is the untouched body, for the action that exposes it.
	Raw map[string]interface{} `json:"-"`
}

// HasSurvey reports whether the template actually asks anything.
func (s SurveySpec) HasSurvey() bool { return len(s.Spec) > 0 }

// VariableNames is the set of variables the survey owns. Answers to these bypass
// ask_variables_on_launch entirely.
func (s SurveySpec) VariableNames() map[string]bool {
	out := make(map[string]bool, len(s.Spec))
	for _, q := range s.Spec {
		if q.Variable != "" {
			out[q.Variable] = true
		}
	}
	return out
}

// RequiredVariables lists the survey variables that must be answered — required
// questions with no usable default.
func (s SurveySpec) RequiredVariables() []string {
	out := []string{}
	for _, q := range s.Spec {
		if q.Required && !hasDefault(q.Default) {
			out = append(out, q.Variable)
		}
	}
	return out
}

// FetchSurveySpec fetches a template's survey.
func FetchSurveySpec(ctx context.Context, a Auth, kind string, id int64) (SurveySpec, error) {
	path, err := TemplateKindPath(kind)
	if err != nil {
		return SurveySpec{}, err
	}
	raw, err := GetResource(ctx, a, fmt.Sprintf("%s%d/survey_spec/", path, id), nil)
	if err != nil {
		return SurveySpec{}, err
	}

	spec := SurveySpec{
		Name:        StringField(raw, "name"),
		Description: StringField(raw, "description"),
		Raw:         raw,
		Spec:        []SurveyQuestion{},
	}
	questions, _ := raw["spec"].([]interface{})
	for _, item := range questions {
		q, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		spec.Spec = append(spec.Spec, SurveyQuestion{
			Variable:            StringField(q, "variable"),
			QuestionName:        StringField(q, "question_name"),
			QuestionDescription: StringField(q, "question_description"),
			Type:                StringField(q, "type"),
			Required:            BoolField(q, "required"),
			Default:             q["default"],
			Choices:             normaliseChoices(q["choices"]),
			Min:                 floatPtr(q["min"]),
			Max:                 floatPtr(q["max"]),
		})
	}
	return spec, nil
}

// ValidateSurvey checks the operator's answers against the survey client-side, so
// a missing or out-of-range answer is reported in the editor rather than coming
// back as an opaque AWX 400.
func ValidateSurvey(spec SurveySpec, extraVars map[string]interface{}) error {
	problems := []string{}

	for _, q := range spec.Spec {
		if q.Variable == "" {
			continue
		}
		answer, answered := extraVars[q.Variable]
		if answered && isBlank(answer) {
			answered = false
		}

		if !answered {
			// AWX fills a defaulted question in for us, so only an unanswered
			// question with no default is a problem.
			if q.Required && !hasDefault(q.Default) {
				label := q.QuestionName
				if label == "" {
					label = q.Variable
				}
				problems = append(problems, fmt.Sprintf("%q is required by this template's survey (%s)", q.Variable, label))
			}
			continue
		}

		if err := validateAnswer(q, answer); err != nil {
			problems = append(problems, err.Error())
		}
	}

	if len(problems) == 0 {
		return nil
	}
	sort.Strings(problems)
	return fmt.Errorf("This job template's survey will not accept these answers: %s. Fix them in Extra Variables / Survey Answers.", strings.Join(problems, "; "))
}

func validateAnswer(q SurveyQuestion, answer interface{}) error {
	switch q.Type {
	case "integer", "float":
		n, ok := numeric(answer)
		if !ok {
			return fmt.Errorf("%q must be a number", q.Variable)
		}
		if q.Type == "integer" && n != float64(int64(n)) {
			return fmt.Errorf("%q must be a whole number", q.Variable)
		}
		if q.Min != nil && n < *q.Min {
			return fmt.Errorf("%q must be at least %s", q.Variable, trimFloat(*q.Min))
		}
		if q.Max != nil && n > *q.Max {
			return fmt.Errorf("%q must be at most %s", q.Variable, trimFloat(*q.Max))
		}

	case "multiplechoice":
		s := fmt.Sprintf("%v", answer)
		if len(q.Choices) > 0 && !containsString(q.Choices, s) {
			return fmt.Errorf("%q must be one of: %s", q.Variable, strings.Join(q.Choices, ", "))
		}

	case "multiselect":
		// A multiselect answer is a JSON ARRAY, unlike multiplechoice's scalar.
		values, ok := answer.([]interface{})
		if !ok {
			return fmt.Errorf("%q must be a list, e.g. [\"%s\"]", q.Variable, firstChoice(q.Choices))
		}
		for _, v := range values {
			s := fmt.Sprintf("%v", v)
			if len(q.Choices) > 0 && !containsString(q.Choices, s) {
				return fmt.Errorf("%q may only contain: %s", q.Variable, strings.Join(q.Choices, ", "))
			}
		}

	case "text", "textarea", "password":
		s := fmt.Sprintf("%v", answer)
		if q.Min != nil && float64(len(s)) < *q.Min {
			return fmt.Errorf("%q must be at least %s characters", q.Variable, trimFloat(*q.Min))
		}
		if q.Max != nil && float64(len(s)) > *q.Max {
			return fmt.Errorf("%q must be at most %s characters", q.Variable, trimFloat(*q.Max))
		}
	}
	return nil
}

// ValidatePrompts refuses, BEFORE launching, any override the template is not
// configured to accept.
//
// ★ This is the safety core of the node. JobTemplateLaunch.post passes
// _exclude_errors=['prompts'], so a prompt field whose ask_* flag is off is NOT
// rejected: the job starts (201), the field is silently DROPPED, and the only
// trace is "ignored_fields" in the response. Sending limit=web* to a template
// with ask_limit_on_launch=false RUNS THE PLAYBOOK AGAINST EVERY HOST IN THE
// INVENTORY. So we fail closed, naming the field and the checkbox to tick.
//
// surveyVars is the set of variable names the template's survey owns (nil when
// there is no survey). Survey answers bypass ask_variables_on_launch entirely —
// only NON-survey extra_vars keys are gated on it.
//
// allowIgnored is the operator's explicit "send it anyway and let AWX drop it"
// escape hatch. It still validates the passwords/inventory pre-conditions, which
// are hard AWX errors rather than silent drops.
func ValidatePrompts(cfg LaunchConfig, body map[string]interface{}, surveyVars map[string]bool, allowIgnored bool) error {
	if !allowIgnored {
		refused := []string{}

		for field := range body {
			if !promptFields[field] || cfg.Ask[field] {
				continue
			}
			if field == "extra_vars" {
				// Survey answers are always allowed; only extra variables the
				// survey does not own need ask_variables_on_launch.
				if extras := nonSurveyKeys(body["extra_vars"], surveyVars); len(extras) > 0 {
					refused = append(refused, fmt.Sprintf(
						"Variables (%s)", strings.Join(extras, ", ")))
				}
				continue
			}
			refused = append(refused, promptLabel(field))
		}

		if len(refused) > 0 {
			sort.Strings(refused)
			return fmt.Errorf(
				"This job template is not configured to accept %s at launch, so AWX would IGNORE what you set and run with the template's own values — which for Limit means running against every host in the inventory. "+
					"Either turn on 'Prompt on launch' for %s in AWX, or clear the field on this node. "+
					"(To send it anyway and let AWX drop it, tick 'Allow Ignored Fields'.)",
				strings.Join(refused, ", "), strings.Join(refused, ", "))
		}
	}

	// Not a silent drop: a hard AWX 400 if we get it wrong, so it is checked even
	// when the operator has ticked Allow Ignored Fields.
	if len(cfg.PasswordsNeededToStart) > 0 {
		given, _ := body["credential_passwords"].(map[string]interface{})
		missing := []string{}
		for _, key := range cfg.PasswordsNeededToStart {
			if v, ok := given[key]; !ok || isBlank(v) {
				missing = append(missing, key)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			return fmt.Errorf(
				"This job template's credentials ask for a password at launch. Fill in Credential Passwords with: %s — for example {\"%s\": \"…\"}.",
				strings.Join(missing, ", "), missing[0])
		}
	}

	if cfg.InventoryNeededToStart && !cfg.Ask["inventory"] {
		if _, given := body["inventory"]; !given {
			return errors.New("This job template has no inventory and is not configured to prompt for one, so it cannot be launched. Set an inventory on the template in AWX, or turn on 'Prompt on launch' for Inventory.")
		}
	}
	if cfg.InventoryNeededToStart && cfg.Ask["inventory"] {
		if v, given := body["inventory"]; !given || isBlank(v) {
			return errors.New("This job template has no inventory of its own, so you must choose one on this node before it can be launched.")
		}
	}

	return nil
}

// ValidateLaunch runs the whole client-side pre-flight for a launch: it fetches
// the template's launch configuration, validates the survey answers, and refuses
// any prompt override the template would silently ignore. The LaunchConfig it
// returns is the same one the caller would have fetched anyway (its Defaults are
// worth surfacing), so a launch action needs exactly one call.
//
// kind is TemplateKindJob or TemplateKindWorkflow. body is the launch body about
// to be POSTed.
func ValidateLaunch(ctx context.Context, a Auth, kind string, id int64, body map[string]interface{}, allowIgnored bool) (LaunchConfig, error) {
	cfg, err := PreflightLaunch(ctx, a, kind, id)
	if err != nil {
		return LaunchConfig{}, err
	}

	var surveyVars map[string]bool
	extraVars, _ := body["extra_vars"].(map[string]interface{})
	if cfg.SurveyEnabled {
		spec, err := FetchSurveySpec(ctx, a, kind, id)
		if err != nil {
			return cfg, err
		}
		surveyVars = spec.VariableNames()
		if err := ValidateSurvey(spec, extraVars); err != nil {
			return cfg, err
		}
	}

	if err := ValidatePrompts(cfg, body, surveyVars, allowIgnored); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// CheckIgnoredFields is the belt-and-braces half of the ignored-fields guard: a
// template can be reconfigured between the pre-flight and the launch, so the 201
// is re-checked. It returns the ignored_fields map (always, so the action can
// emit it) plus an error when AWX dropped something and the operator did not ask
// for that.
func CheckIgnoredFields(launched map[string]interface{}, allowIgnored bool) (map[string]interface{}, error) {
	ignored, _ := launched["ignored_fields"].(map[string]interface{})
	if len(ignored) == 0 || allowIgnored {
		return ignored, nil
	}
	fields := make([]string, 0, len(ignored))
	for field := range ignored {
		fields = append(fields, promptLabel(field))
	}
	sort.Strings(fields)
	return ignored, fmt.Errorf(
		"AWX accepted the launch but IGNORED %s — the template is not configured to prompt for it, so the job would have run with the template's own values instead of yours. The job has been started; check it in AWX. Turn on 'Prompt on launch' for %s in AWX, or clear the field on this node.",
		strings.Join(fields, ", "), strings.Join(fields, ", "))
}

// LaunchedJob reads the id and kind of the job a launch, relaunch or sync just
// created. AWX is inconsistent here in four different ways, so every caller uses
// this rather than reaching into the body:
//
//   - a job-template launch answers {"job": 42, "type": "job", …};
//   - ★ a SLICED job template (job_slice_count > 1) answers a WORKFLOW JOB —
//     {"workflow_job": 99, "type": "workflow_job", …} with NO "job" key at all —
//     and polling /jobs/99/ for it gives a 404 the operator cannot explain;
//   - an ad-hoc relaunch answers {"ad_hoc_command": …}, a project sync
//     {"project_update": …}, an inventory sync {"inventory_update": …};
//   - a WORKFLOW relaunch adds no extra key at all: the new id is in "id".
//
// fallbackKind is used when the response carries no type (pass the kind you
// expected).
func LaunchedJob(launched map[string]interface{}, fallbackKind string) (int64, string, error) {
	kind := StringField(launched, "type")
	if _, ok := JobKindPaths[kind]; !ok {
		kind = fallbackKind
	}
	if _, ok := JobKindPaths[kind]; !ok {
		return 0, "", fmt.Errorf("AWX returned a job of an unexpected kind (%q)", StringField(launched, "type"))
	}

	for _, key := range []string{kind, "id"} {
		if id, ok := int64Value(launched[key]); ok && id > 0 {
			return id, kind, nil
		}
	}
	return 0, "", errors.New("AWX started the job but did not report its ID")
}

// ---------------------------------------------------------------------------
// Jobs: paths, waiting, output
// ---------------------------------------------------------------------------

// JobKindPaths maps AWX's five unified-job kinds to their collection path. The
// record shape and the status/finished semantics are identical across all five,
// which is why one Get Job action covers them all.
var JobKindPaths = map[string]string{
	JobKindJob:             "jobs/",
	JobKindWorkflowJob:     "workflow_jobs/",
	JobKindAdHocCommand:    "ad_hoc_commands/",
	JobKindProjectUpdate:   "project_updates/",
	JobKindInventoryUpdate: "inventory_updates/",
}

// JobKindPath resolves a job kind to its collection path. An empty kind means the
// default, a plain job.
func JobKindPath(kind string) (string, error) {
	if kind == "" {
		kind = JobKindJob
	}
	path, ok := JobKindPaths[kind]
	if !ok {
		return "", fmt.Errorf("unknown job type %q — choose Job, Workflow Job, Ad-Hoc Command, Project Sync or Inventory Sync", kind)
	}
	return path, nil
}

// TemplateKindPath resolves a launchable template kind to its collection path. An
// empty kind means a job template.
func TemplateKindPath(kind string) (string, error) {
	switch kind {
	case "", TemplateKindJob:
		return "job_templates/", nil
	case TemplateKindWorkflow:
		return "workflow_job_templates/", nil
	default:
		return "", fmt.Errorf("unknown template type %q", kind)
	}
}

// jobUIPaths maps a job kind to the AWX web UI's route for it, so an action can
// hand the operator a link they can actually click.
var jobUIPaths = map[string]string{
	JobKindJob:             "playbook",
	JobKindWorkflowJob:     "workflow",
	JobKindAdHocCommand:    "command",
	JobKindProjectUpdate:   "project",
	JobKindInventoryUpdate: "inventory",
}

// JobURL builds the AWX UI link for a job.
func JobURL(a Auth, kind string, id int64) string {
	segment, ok := jobUIPaths[kind]
	if !ok {
		segment = "playbook"
	}
	return fmt.Sprintf("%s/#/jobs/%s/%d/output", a.BaseURL, segment, id)
}

// WaitOpts configures WaitForJob.
type WaitOpts struct {
	// PollIntervalSeconds is a FLOOR, not a fixed period: see pollInterval. <= 0
	// means DefaultPollSeconds.
	PollIntervalSeconds int
	// TimeoutSeconds <= 0 means DefaultWaitSeconds. It is hard-capped at
	// MaxWaitSeconds, because a waiting node pins a flow worker for its whole
	// duration.
	TimeoutSeconds int
	// CancelOnTimeout cancels the AWX job when the wait runs out. Off by default:
	// silently killing a production job because a flow got bored is surprising
	// and destructive.
	CancelOnTimeout bool
	// IncludeStdout fetches the job's output once its events have settled. It
	// implies WaitForEvents.
	IncludeStdout  bool
	StdoutMaxBytes int
	// WaitForEvents holds on past the terminal status until AWX has finished
	// writing the job's events, so artifacts and host_status_counts are complete.
	WaitForEvents bool
}

// WaitResult is what a wait produced. Job is the job's record — the DETAIL view
// when the job went terminal (result_traceback, event_processing_finished and
// host_status_counts exist only there), or the last list record seen if the wait
// timed out.
type WaitResult struct {
	Job             map[string]interface{}
	TimedOut        bool
	Canceled        bool // the wait timed out and CancelOnTimeout cancelled the job
	EventsSettled   bool
	Stdout          string
	StdoutTruncated bool
}

// WaitForJob polls an AWX job to completion.
//
// There is no long-poll, no ?wait and no blocking endpoint: polling is the
// supported and universal approach (it is what ansible.controller.job_wait does).
// The WebSocket at wss://host/websocket/ authenticates fine with a bearer token
// but can never subscribe — EventConsumer.receive_json requires an xrftoken
// matching a csrftoken COOKIE, which a token client does not have, so every
// subscribe returns "access denied to channel" and you hold an open socket that
// delivers nothing. Do not attempt it.
//
// Three things this gets right that a naive loop does not:
//
//   - It polls the LIST endpoint (?id=N), not the detail. JobDetailSerializer
//     computes playbook_counts with two COUNT(*) queries over the job-events
//     table on EVERY detail GET, so 1 Hz detail polling on a 500k-event job is
//     genuinely expensive for the AWX database. One detail GET is taken at the
//     end, for the fields the list view strips.
//   - The terminal test is `finished != null` — a timestamp, immune to any future
//     status being added — not a status allow-list.
//   - It gates on event_processing_finished before reading stdout or artifacts.
//     AWX writes job events to Postgres asynchronously: status flips to
//     successful the instant the runner exits, but the output may still be
//     flushing, so reading it immediately yields TRUNCATED OR EMPTY results.
//
// ★ The wait is bounded by opts.TimeoutSeconds and DELIBERATELY DOES NOT INHERIT
// the caller's deadline. Every context in an action is manufactured by the action
// itself (core.Action takes none), so there is no upstream cancellation to honour
// — but there IS an easy mistake to make: an action that passed the ordinary
// 75-second Context() would otherwise have its 10-minute wait cut short at 75
// seconds and report a perfectly healthy job as timed out.
func WaitForJob(ctx context.Context, a Auth, kind string, id int64, opts WaitOpts) (*WaitResult, error) {
	path, err := JobKindPath(kind)
	if err != nil {
		return nil, err
	}
	if kind == "" {
		kind = JobKindJob
	}

	if ctx == nil {
		ctx = context.Background()
	}
	parent := context.WithoutCancel(ctx)
	timeout := time.Duration(ClampWaitSeconds(opts.TimeoutSeconds)) * time.Second
	pollCtx, cancelPoll := context.WithTimeout(parent, timeout)
	defer cancelPoll()

	res := &WaitResult{}
	start := time.Now()
	var last map[string]interface{}
	terminal := false

	for {
		rec, err := fetchJobSummary(pollCtx, a, path, id)
		if err != nil {
			if pollCtx.Err() != nil {
				break // the deadline landed mid-request
			}
			return nil, err
		}
		last = rec
		if isFinished(rec) {
			terminal = true
			break
		}
		if pollCtx.Err() != nil {
			break
		}

		timer := time.NewTimer(pollInterval(opts.PollIntervalSeconds, time.Since(start)))
		select {
		case <-pollCtx.Done():
			timer.Stop()
		case <-timer.C:
		}
		if pollCtx.Err() != nil {
			break
		}
	}

	res.Job = last
	res.TimedOut = !terminal

	if res.TimedOut {
		// The poll context is spent; everything from here uses the caller's.
		if opts.CancelOnTimeout {
			if _, err := CancelJob(parent, a, kind, id); err == nil {
				res.Canceled = true
			}
		}
		if detail, err := GetResource(parent, a, fmt.Sprintf("%s%d/", path, id), nil); err == nil {
			res.Job = detail
		}
		return res, nil
	}

	// One detail GET for the terminal record: result_traceback,
	// event_processing_finished and host_status_counts are stripped from the list
	// view and exist only here.
	detail, err := GetResource(parent, a, fmt.Sprintf("%s%d/", path, id), nil)
	if err != nil {
		return nil, err
	}
	res.Job = detail

	wantEvents := opts.WaitForEvents || opts.IncludeStdout
	// A workflow job is a pure orchestration record: it has no events, no stdout
	// and no artifacts of its own. Everything real lives on its child jobs.
	if wantEvents && kind != JobKindWorkflowJob {
		settled, rec := waitForEvents(parent, a, path, id, detail)
		res.EventsSettled = settled
		res.Job = rec
	}

	if opts.IncludeStdout && kind != JobKindWorkflowJob {
		text, truncated, err := FetchStdout(parent, a, kind, id, opts.StdoutMaxBytes)
		if err != nil {
			return nil, err
		}
		res.Stdout = text
		res.StdoutTruncated = truncated || !res.EventsSettled
	}

	return res, nil
}

// fetchJobSummary reads a job's current state from the LIST endpoint — cheap for
// AWX, unlike the detail view.
func fetchJobSummary(ctx context.Context, a Auth, path string, id int64) (map[string]interface{}, error) {
	q := url.Values{}
	q.Set("id", strconv.FormatInt(id, 10))
	items, _, _, err := List(ctx, a, path, q, false)
	if err != nil {
		return nil, err
	}
	if len(items) == 0 {
		return nil, fmt.Errorf("AWX has no %s with ID %d", strings.ReplaceAll(strings.TrimSuffix(path, "s/"), "_", " "), id)
	}
	rec, ok := items[0].(map[string]interface{})
	if !ok {
		return nil, errors.New("AWX returned an unexpected shape for the job")
	}
	return rec, nil
}

// waitForEvents holds on until AWX reports it has finished writing the job's
// events. It is capped: after eventSettleTimeout we proceed anyway and let the
// caller flag the output as possibly incomplete, rather than hanging the flow.
func waitForEvents(ctx context.Context, a Auth, path string, id int64, current map[string]interface{}) (bool, map[string]interface{}) {
	if BoolField(current, "event_processing_finished") {
		return true, current
	}

	deadline := time.Now().Add(eventSettleTimeout)
	rec := current
	for time.Now().Before(deadline) {
		timer := time.NewTimer(eventSettleInterval)
		select {
		case <-ctx.Done():
			timer.Stop()
			return false, rec
		case <-timer.C:
		}

		next, err := GetResource(ctx, a, fmt.Sprintf("%s%d/", path, id), nil)
		if err != nil {
			return false, rec
		}
		rec = next
		if BoolField(rec, "event_processing_finished") {
			return true, rec
		}
	}
	return false, rec
}

// isFinished is the terminal test: a non-null finished TIMESTAMP. Equivalent to
// status ∉ {new, pending, waiting, running} but immune to a status being added.
func isFinished(job map[string]interface{}) bool {
	v, ok := job["finished"]
	if !ok || v == nil {
		return false
	}
	s, ok := v.(string)
	return !ok || strings.TrimSpace(s) != ""
}

// pollInterval is the operator's interval used as a FLOOR, stretched on a long
// wait so a job that runs for an hour does not have us hammering the AWX database
// every few seconds for it: 2s minimum, 5s after 30 seconds, 10s after 2 minutes.
func pollInterval(configuredSeconds int, elapsed time.Duration) time.Duration {
	floor := time.Duration(configuredSeconds) * time.Second
	if configuredSeconds <= 0 {
		floor = DefaultPollSeconds * time.Second
	}

	backoff := 2 * time.Second
	switch {
	case elapsed > 2*time.Minute:
		backoff = 10 * time.Second
	case elapsed > 30*time.Second:
		backoff = 5 * time.Second
	}
	if backoff > floor {
		return backoff
	}
	return floor
}

// ClampWaitSeconds bounds a wait to something a flow worker can afford to be
// pinned for, defaulting a blank value.
func ClampWaitSeconds(seconds int) int {
	if seconds <= 0 {
		return DefaultWaitSeconds
	}
	if seconds > MaxWaitSeconds {
		return MaxWaitSeconds
	}
	return seconds
}

// CancelJob asks AWX to cancel a job. It reports alreadyFinished=true (with a nil
// error) when the job had already gone terminal.
//
// Three traps, all handled here:
//
//   - POSTing /cancel/ on an ALREADY-FINISHED job answers 405 Method Not Allowed,
//     not 409 or 400. That is "already terminal", not a routing bug.
//   - Success answers 202 with a COMPLETELY EMPTY body — there is nothing to
//     parse off it.
//   - Cancellation is ASYNCHRONOUS: cancel() only sets cancel_flag and notifies
//     the dispatcher, so the status may still read running for seconds afterwards
//     — and the job can still land on successful if it finished in the race.
func CancelJob(ctx context.Context, a Auth, kind string, id int64) (alreadyFinished bool, err error) {
	path, err := JobKindPath(kind)
	if err != nil {
		return false, err
	}
	resp, err := Do(ctx, a, http.MethodPost, fmt.Sprintf("%s%d/cancel/", path, id), nil)
	if err != nil {
		return false, err
	}
	if resp.StatusCode == http.StatusMethodNotAllowed {
		return true, nil
	}
	if err := CheckResponse(a, resp, http.StatusAccepted); err != nil {
		return false, err
	}
	return false, nil
}

// DecodeArtifacts normalises a job's artifacts.
//
// job.artifacts is EITHER a JSON object OR the literal string
// "$hidden due to Ansible no_log flag$" (UnifiedJob.display_artifacts).
// Unmarshalling straight into map[string]interface{} FAILS on any job whose
// set_stats was no_log — so decode into any, and hand back whatever was there.
func DecodeArtifacts(raw interface{}) interface{} {
	switch v := raw.(type) {
	case nil:
		return map[string]interface{}{}
	case map[string]interface{}:
		return v
	case string:
		// The no_log sentinel (or any other string AWX chooses to put here).
		return v
	default:
		return v
	}
}

// FetchStdout fetches a job's plain-text playbook output.
//
// ALWAYS ?format=txt_download. ?format=txt is capped at STDOUT_MAX_BYTES_DISPLAY
// (1 MiB) and — this is the trap — a job over the cap answers HTTP 200 WHOSE BODY
// IS THE ENGLISH SENTENCE "Standard Output too large to display (N bytes), only
// download supported for sizes over 1048576 bytes.", which a naive client stores
// AS the playbook output. txt_download is uncapped, streams from disk, is still
// text/plain, and we cap client-side. Paging with start_line/end_line does NOT get
// around the cap — it is checked on the whole job before slicing.
//
// The ?format= parameter is also what stops AWX serving an HTML page:
// BrowsableAPIRenderer is first in renderer_classes, so a bare GET with
// Accept: */* returns the browsable API, not the log.
func FetchStdout(ctx context.Context, a Auth, kind string, id int64, maxBytes int) (text string, truncated bool, err error) {
	if kind == "" {
		kind = JobKindJob
	}
	if kind == JobKindWorkflowJob {
		return "", false, errors.New("a workflow job has no output of its own — list its nodes and fetch the output of the child job you want")
	}
	path, err := JobKindPath(kind)
	if err != nil {
		return "", false, err
	}
	root, err := ResolveAPIRoot(ctx, a)
	if err != nil {
		return "", false, err
	}

	target := fmt.Sprintf("%s%s%s%d/stdout/?format=txt_download", a.BaseURL, root.Prefix, path, id)
	resp, err := request(ctx, a, http.MethodGet, target, nil, "text/plain", true)
	if err != nil {
		return "", false, err
	}
	if err := CheckResponse(a, resp); err != nil {
		return "", false, err
	}

	text = string(resp.Body)
	truncated = resp.Truncated

	// Belt and braces: never store AWX's "too large" apology as if it were the
	// playbook's output.
	if strings.HasPrefix(strings.TrimSpace(text), stdoutTooLargeSentence) {
		return "", false, fmt.Errorf("AWX would not return this job's output inline: %s", strings.TrimSpace(text))
	}

	if maxBytes <= 0 {
		maxBytes = DefaultStdoutMaxBytes
	}
	if len(text) > maxBytes {
		text = strings.ToValidUTF8(text[:maxBytes], "")
		truncated = true
	}
	return text, truncated, nil
}

// JobOutputs flattens a job record into the outputs every job-shaped action
// emits, so the launch, wait, relaunch, sync and get actions cannot drift from
// one another.
//
// failed is true for BOTH a failed job (hosts failed) and an errored one (AWX
// could not run it) — only the latter populates result_traceback, so the two are
// worth distinguishing in an action's tool_result.
func JobOutputs(a Auth, kind string, job map[string]interface{}) map[string]interface{} {
	if job == nil {
		job = map[string]interface{}{}
	}
	if kind == "" {
		kind = JobKindJob
	}
	id, _ := int64Value(job["id"])

	return map[string]interface{}{
		"job_id":                    IDString(job["id"]),
		"job_kind":                  kind,
		"status":                    StringField(job, "status"),
		"finished":                  isFinished(job),
		"failed":                    BoolField(job, "failed"),
		"elapsed":                   IDString(job["elapsed"]),
		"artifacts":                 DecodeArtifacts(job["artifacts"]),
		"host_status_counts":        job["host_status_counts"],
		"job_explanation":           StringField(job, "job_explanation"),
		"result_traceback":          StringField(job, "result_traceback"),
		"event_processing_finished": BoolField(job, "event_processing_finished"),
		"job_url":                   JobURL(a, kind, id),
		"result":                    job,
	}
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// OptionalString extracts a string input, returning "" when absent.
//
// The explicit Value == nil guard matters for the Secret and Code input types
// (api_token, awx_password): unlike a String, their String() renders a nil value
// as the literal "<nil>" rather than "", so an unset API token would otherwise
// read back as non-empty and sail through the required-field check.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}

// RequiredString extracts a required string input. The error names the human
// Label, because that is what the operator sees on the node.
func RequiredString(name, label string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return v, nil
}

// OptionalInt extracts an integer input. The bool is false when the input is
// absent or unusable, so callers can tell "unset" from "set to 0".
//
// It does not call core.Connection.Number(), which only reads Integer-typed
// inputs: an AWX object ID is carried on a STRING input (a live dropdown writes a
// string), so Number() would report every id as unset. It also refuses to panic
// on a whole-value ${...} reference that lands a slice or a map in a numeric
// field — that reads as unset rather than crashing the one-shot executor.
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil {
		return 0, false
	}
	n, ok := int64Value(conn.Value)
	return int(n), ok
}

// RequiredInt extracts a required AWX object ID. It accepts the string a live
// dropdown writes as well as a real integer.
func RequiredInt(name, label string, inputs []*core.Connection) (int64, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil || OptionalString(name, inputs) == "" {
		return 0, fmt.Errorf("%s is required", label)
	}
	n, ok := int64Value(conn.Value)
	if !ok {
		return 0, fmt.Errorf("%s must be a whole number — the AWX ID of the object, e.g. 7", label)
	}
	return n, nil
}

// BoolInput reads a boolean input, coercing a string value.
//
// It deliberately does NOT lean on core.Connection.Boolean(): the editor stores a
// variable-bound checkbox as the STRING "${var.approved}", and the flow engine's
// substitution pass rewrites every ${...} into a string before the action ever
// sees it — so a checkbox bound to a variable arrives as the string "true".
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

// ConfirmDestructive gates an action that permanently changes AWX state.
//
// It fails closed: an unset, blank or unparseable value refuses the action. An
// unresolvable ${var.x} substitutes to the empty string, so a typo'd variable
// name declines to delete the inventory rather than deleting it.
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
// rather than an array or a scalar. Returns (nil, nil) when absent.
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

// SetIfPresent adds an optional string field to a request body only when the
// input was filled in, so unset fields are omitted rather than blanked.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// SetIntIfPresent adds an integer field when present. AWX wants true JSON
// integers for every id and count field.
func SetIntIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) error {
	conn := core.FindConnection(inputName, inputs)
	if conn == nil || conn.Value == nil || OptionalString(inputName, inputs) == "" {
		return nil
	}
	n, ok := int64Value(conn.Value)
	if !ok {
		return fmt.Errorf("%s must be a whole number", inputName)
	}
	body[field] = n
	return nil
}

// SetBoolIfSet adds a boolean field ONLY when its input was actually touched, so
// the tri-state nil is preserved as "omit".
//
// This matters wherever AWX's own default is true — a host's `enabled`, a
// schedule's `enabled`. The manifest cannot carry a default Value, so those
// checkboxes render unticked; if an untouched checkbox sent false, adding a host
// would silently disable it.
func SetBoolIfSet(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	conn := core.FindConnection(inputName, inputs)
	if conn == nil || conn.Value == nil {
		return
	}
	// An unresolved ${var.x} substitutes to "" — that is "untouched", not "false".
	if s, ok := conn.Value.(string); ok && strings.TrimSpace(s) == "" {
		return
	}
	body[field] = BoolInput(inputName, inputs)
}

// SetJSONIfPresent adds a parsed JSON input to the body when present.
func SetJSONIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) error {
	v, err := OptionalJSON(inputName, inputs)
	if err != nil {
		return err
	}
	if v != nil {
		body[field] = v
	}
	return nil
}

// SetIntListIfPresent maps a comma-separated list of AWX ids (or a JSON array of
// them) to the plain [N] integer-array shape AWX wants for credentials, labels and
// instance_groups. Accepts "1,4" or "[1,4]".
func SetIntListIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	raw := OptionalString(inputName, inputs)
	if raw == "" {
		return
	}
	raw = strings.Trim(raw, "[] ")
	ids := []interface{}{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.Trim(strings.TrimSpace(part), `"'`)
		if part == "" {
			continue
		}
		if n, err := strconv.ParseInt(part, 10, 64); err == nil {
			ids = append(ids, n)
		}
	}
	if len(ids) > 0 {
		body[field] = ids
	}
}

// SetStringListIfPresent maps a comma-separated list to a []string body field.
func SetStringListIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	raw := OptionalString(inputName, inputs)
	if raw == "" {
		return
	}
	vals := []interface{}{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			vals = append(vals, p)
		}
	}
	if len(vals) > 0 {
		body[field] = vals
	}
}

// MergeAdditionalFields overlays the raw "additional_fields" JSON object onto a
// request body — the escape hatch for any AWX field this node does not expose as
// a first-class input.
//
// It is called LAST in every action's body assembly, so a key here OVERRIDES the
// same key set by a first-class input. That "power-user last word" precedence is
// deliberate and matches the WordPress / WooCommerce / Cal.com nodes.
func MergeAdditionalFields(body map[string]interface{}, inputs []*core.Connection) error {
	obj, err := OptionalJSONObject("additional_fields", inputs)
	if err != nil {
		return err
	}
	for k, v := range obj {
		body[k] = v
	}
	return nil
}

// AddFilter sets a query param from an optional string input when it is non-empty.
func AddFilter(q url.Values, inputs []*core.Connection, param, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		q.Set(param, v)
	}
}

// ClampPageSize bounds a requested page size to AWX's 1..MaxPageSize range,
// falling back to DefaultPageSize when unset. AWX clamps silently, so we do it
// visibly.
func ClampPageSize(size int, set bool) int {
	if !set || size <= 0 {
		return DefaultPageSize
	}
	if size > MaxPageSize {
		return MaxPageSize
	}
	return size
}

// ListParams builds the page / page_size / order_by query every *_list action
// shares, and reports whether Return All was ticked. defaultOrderBy is applied
// when the operator leaves Order By blank ("" to send none) — AWX's own default
// ordering on the job collections is id ASCENDING, i.e. oldest first, which is
// never what a human wants.
func ListParams(inputs []*core.Connection, defaultOrderBy string) (url.Values, bool) {
	q := url.Values{}

	size, set := OptionalInt("page_size", inputs)
	q.Set("page_size", strconv.Itoa(ClampPageSize(size, set)))

	if page, ok := OptionalInt("page", inputs); ok && page > 1 {
		q.Set("page", strconv.Itoa(page))
	}

	order := OptionalString("order_by", inputs)
	if order == "" {
		order = defaultOrderBy
	}
	if order != "" {
		q.Set("order_by", order)
	}

	return q, BoolInput("return_all", inputs)
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ObjectResult shapes a single-object response into the standard action output.
func ObjectResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          IDString(obj["id"]),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response. total is AWX's count — the number of
// rows MATCHING on the server, which is not the number returned — so both are
// emitted: count is len(results), total_count is the server's total.
func ListResult(items []interface{}, total int, hasMore bool, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"total_count": total,
		"has_more":    hasMore,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// SuccessResult is the standard output for an action whose signal is that it
// worked (launch, cancel, attach, sync). extra keys are merged in.
func SuccessResult(summary string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// ErrorResult is the standard SOFT-failure output map. It is returned alongside a
// NIL error, so the flow engine marks the node failed in the UI, caches the
// outputs and KEEPS WALKING the flow — which is what lets an AI tool loop read
// the error and recover. A non-nil Go error aborts the entire flow run, and is
// reserved for GetAuth.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ---------------------------------------------------------------------------
// Small shared conversions
// ---------------------------------------------------------------------------

// IDString renders an AWX identifier as a string. AWX ids are JSON NUMBERS, so a
// naive v.(string) yields "" and the action emits an empty id — hence the type
// switch. It is also what renders the float elapsed seconds.
func IDString(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case float32:
		return IDString(float64(t))
	case int:
		return strconv.Itoa(t)
	case int32:
		return strconv.FormatInt(int64(t), 10)
	case int64:
		return strconv.FormatInt(t, 10)
	case json.Number:
		return t.String()
	case bool:
		return strconv.FormatBool(t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// StringField reads a string field out of a decoded AWX object, "" when it is
// absent or null.
func StringField(obj map[string]interface{}, key string) string {
	if obj == nil {
		return ""
	}
	v, ok := obj[key]
	if !ok || v == nil {
		return ""
	}
	if s, ok := v.(string); ok {
		return s
	}
	return IDString(v)
}

// BoolField reads a boolean field out of a decoded AWX object, false when absent.
func BoolField(obj map[string]interface{}, key string) bool {
	if obj == nil {
		return false
	}
	switch v := obj[key].(type) {
	case bool:
		return v
	case string:
		b, err := strconv.ParseBool(strings.TrimSpace(v))
		return err == nil && b
	default:
		return false
	}
}

// int64Value coerces the many shapes an integer arrives in — a JSON float64, an
// auto-wired int/int64 from an upstream node, or the string a live dropdown or a
// resolved ${...} reference writes.
func int64Value(v interface{}) (int64, bool) {
	switch t := v.(type) {
	case int:
		return int64(t), true
	case int32:
		return int64(t), true
	case int64:
		return t, true
	case float64:
		n := int64(t)
		if float64(n) != t {
			return 0, false
		}
		return n, true
	case float32:
		return int64Value(float64(t))
	case json.Number:
		n, err := t.Int64()
		return n, err == nil
	case string:
		s := strings.TrimSpace(t)
		if s == "" {
			return 0, false
		}
		if n, err := strconv.ParseInt(s, 10, 64); err == nil {
			return n, true
		}
		// Tolerate "7.0" — a spreadsheet or a JSON round-trip can produce it.
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			n := int64(f)
			if float64(n) == f {
				return n, true
			}
		}
		return 0, false
	default:
		return 0, false
	}
}

// numeric coerces a survey answer to a float for min/max checking.
func numeric(v interface{}) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case float32:
		return float64(t), true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case json.Number:
		f, err := t.Float64()
		return f, err == nil
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	default:
		return 0, false
	}
}

// stringList reads a JSON array of strings, tolerating a single string.
func stringList(v interface{}) []string {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			if s := IDString(item); s != "" {
				out = append(out, s)
			}
		}
		return out
	case string:
		if strings.TrimSpace(t) == "" {
			return []string{}
		}
		return []string{t}
	default:
		return []string{}
	}
}

// normaliseChoices renders a survey question's choices as an array. AWX serves
// them as a NEWLINE-SEPARATED STRING ("dev\nstaging\nprod") on some versions and
// as a real array on others.
func normaliseChoices(v interface{}) []string {
	switch t := v.(type) {
	case []interface{}:
		out := make([]string, 0, len(t))
		for _, item := range t {
			out = append(out, IDString(item))
		}
		return out
	case string:
		if strings.TrimSpace(t) == "" {
			return nil
		}
		out := []string{}
		for _, line := range strings.Split(t, "\n") {
			if s := strings.TrimSpace(line); s != "" {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}

// nonSurveyKeys lists the extra_vars keys the template's survey does NOT own —
// the ones that genuinely need ask_variables_on_launch.
func nonSurveyKeys(extraVars interface{}, surveyVars map[string]bool) []string {
	vars, ok := extraVars.(map[string]interface{})
	if !ok || len(vars) == 0 {
		return nil
	}
	out := []string{}
	for key := range vars {
		if !surveyVars[key] {
			out = append(out, key)
		}
	}
	sort.Strings(out)
	return out
}

// promptLabel names a prompt field the way AWX's own UI names it.
func promptLabel(field string) string {
	if label, ok := promptLabels[field]; ok {
		return label
	}
	return field
}

func floatPtr(v interface{}) *float64 {
	f, ok := numeric(v)
	if !ok {
		return nil
	}
	return &f
}

func hasDefault(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(t) != ""
	case []interface{}:
		return len(t) > 0
	default:
		return true
	}
}

func isBlank(v interface{}) bool {
	switch t := v.(type) {
	case nil:
		return true
	case string:
		return strings.TrimSpace(t) == ""
	default:
		return false
	}
}

func containsString(list []string, want string) bool {
	for _, s := range list {
		if s == want {
			return true
		}
	}
	return false
}

func firstChoice(choices []string) string {
	if len(choices) == 0 {
		return "value"
	}
	return choices[0]
}

func trimFloat(f float64) string {
	if f == float64(int64(f)) {
		return strconv.FormatInt(int64(f), 10)
	}
	return strconv.FormatFloat(f, 'f', -1, 64)
}
