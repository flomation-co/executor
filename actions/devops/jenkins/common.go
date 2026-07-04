// Package jenkins holds the shared HTTP client, auth helper, and generic
// request/result plumbing used by every devops/jenkins/* action.
//
// Jenkins exposes a uniform REST API rooted at the instance URL: metadata reads
// are GET /…/api/json, and actions (trigger a build, restart, delete a job) are
// parameterless POSTs to a well-known path. That regularity lets the request,
// crumb handling and result shaping live here once so each action package stays
// thin: read its inputs, call one helper, shape the result.
//
// Auth is HTTP Basic — a username plus a personal API token. Two Jenkins quirks
// are handled here so the actions don't have to:
//
//   - Job paths. A job named "A" lives at /job/A; a job "B" nested in folder "A"
//     lives at /job/A/job/B. JobPath turns either the bare name or a "A/B" path
//     into the correct /job/…/job/… URL, so folder-nested jobs work too — a step
//     up from n8n's flat /job/{name}.
//   - CSRF crumbs. Requests authenticated with an API token are exempt from
//     Jenkins' crumb requirement, so the common case needs no crumb (matching
//     n8n). Hardened or password-authenticated instances still enforce it, so a
//     mutating request that comes back 403-because-of-a-crumb is transparently
//     retried once with a freshly-issued crumb.
package jenkins

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// maxResponseBody caps a response body to prevent memory exhaustion. Console
	// logs can be large, so 8 MB rather than 1 MB.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single Jenkins call.
	requestTimeout = 30 * time.Second
)

// httpClient is shared across every Jenkins action so TCP connections to an
// instance are pooled and reused rather than re-dialled per call.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// ---------------------------------------------------------------------------
// Credentials
// ---------------------------------------------------------------------------

// Config is a resolved set of Jenkins credentials for one action invocation.
type Config struct {
	BaseURL  string // normalised instance URL, no trailing slash (may carry a context path)
	Username string
	Token    string
}

// AuthInputs is the shared credential trio every action puts first. Action
// packages re-declare their own literal Inputs arrays (the manifest generator
// AST-parses those); this documents the canonical shape and order.
var AuthInputs = [...]core.Connection{
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "Jenkins URL", Placeholder: "https://jenkins.example.com", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "your Jenkins username", Required: true},
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "a Jenkins API token (User ▸ Configure ▸ API Token)", Required: true},
}

// GetConfig resolves and validates the base_url / username / api_token inputs.
func GetConfig(inputs []*core.Connection) (Config, error) {
	base, err := NormaliseBaseURL(OptionalString("base_url", inputs))
	if err != nil {
		return Config{}, err
	}
	user := OptionalString("username", inputs)
	if user == "" {
		return Config{}, fmt.Errorf("Username is required")
	}
	token := OptionalString("api_token", inputs)
	if token == "" {
		return Config{}, fmt.Errorf("API Token is required — connect a Jenkins credential or supply an API token")
	}
	return Config{BaseURL: base, Username: user, Token: token}, nil
}

// NormaliseBaseURL trims and validates a pasted instance URL into a bare base
// with no trailing slash. A bare host is tolerated (https is assumed); a full
// http(s) URL — including one served under a context path like
// https://host/jenkins — is preserved. Query/fragment are stripped so a crafted
// value can't smuggle a query string onto every request.
func NormaliseBaseURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("Jenkins URL is required")
	}
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" {
		return "", fmt.Errorf("Jenkins URL must be a full http(s) URL, e.g. https://jenkins.example.com")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("Jenkins URL must start with http:// or https://")
	}
	u.Path = strings.TrimRight(u.Path, "/")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}

// JobPath turns a job name or "folder/job" path into the Jenkins /job/… URL
// prefix. "A" → "/job/A"; "A/B" → "/job/A/job/B". Each segment is path-escaped.
// A leading/trailing slash is tolerated. Returns "" for an empty name.
func JobPath(job string) string {
	job = strings.Trim(strings.TrimSpace(job), "/")
	if job == "" {
		return ""
	}
	var b strings.Builder
	for _, seg := range strings.Split(job, "/") {
		if seg == "" {
			continue
		}
		b.WriteString("/job/")
		b.WriteString(url.PathEscape(seg))
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// Response is the raw outcome of a Jenkins call. Location carries the queue-item
// URL that a build trigger returns in its 201 Location header. Truncated is set
// when the body hit the maxResponseBody cap, so a caller (e.g. a console-log
// fetch) can signal that the data is incomplete rather than reporting a clean
// success on a silently-clipped log.
type Response struct {
	StatusCode int
	Body       []byte
	Location   string
	Header     http.Header
	Truncated  bool
}

// Get performs a GET against the instance. path is rooted at the base URL and
// must include any query string (e.g. "/api/json?tree=jobs[name]").
func Get(cfg Config, path string) (*Response, error) {
	return do(httpClient, cfg, http.MethodGet, path, "", nil)
}

// Post performs a POST against the instance, transparently supplying a CSRF
// crumb on the retry if the instance demands one. contentType/body are empty for
// the many parameterless action endpoints; a form or XML body is passed for the
// build-with-parameters and create/copy-job endpoints respectively.
func Post(cfg Config, path, contentType string, body []byte) (*Response, error) {
	resp, err := do(httpClient, cfg, http.MethodPost, path, contentType, body)
	if err != nil {
		return nil, err
	}
	// API-token auth is crumb-exempt, so this retry only fires on hardened or
	// password-authenticated instances. Detect the crumb rejection by status +
	// body rather than assuming it, so an unrelated 403 surfaces as itself.
	if resp.StatusCode == http.StatusForbidden && strings.Contains(strings.ToLower(string(resp.Body)), "crumb") {
		// Jenkins binds the crumb to the HTTP session, so the crumb fetch and
		// the retried POST must share a cookie jar to carry the JSESSIONID
		// between them — a crumb sent without its session cookie is still
		// rejected. A per-call jar keeps sessions from leaking between actions.
		jar, _ := cookiejar.New(nil)
		client := &http.Client{Timeout: requestTimeout, Transport: httpClient.Transport, Jar: jar}
		if field, crumb, ok := fetchCrumb(client, cfg); ok {
			return do(client, cfg, http.MethodPost, path, contentType, body, header{field, crumb})
		}
	}
	return resp, nil
}

type header struct{ key, value string }

func do(client *http.Client, cfg Config, method, path, contentType string, body []byte, headers ...header) (*Response, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequest(method, cfg.BaseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Basic "+basicAuth(cfg.Username, cfg.Token))
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for _, h := range headers {
		req.Header.Set(h.key, h.value)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Jenkins request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	// Read one byte past the cap so an exactly-at-cap body is distinguishable
	// from a clipped one: if we got more than maxResponseBody, the response was
	// larger and is truncated back to the cap.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	truncated := false
	if len(respBody) > maxResponseBody {
		respBody = respBody[:maxResponseBody]
		truncated = true
	}
	return &Response{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Location:   resp.Header.Get("Location"),
		Header:     resp.Header,
		Truncated:  truncated,
	}, nil
}

func basicAuth(user, token string) string {
	return base64.StdEncoding.EncodeToString([]byte(user + ":" + token))
}

// fetchCrumb asks the crumb issuer for a CSRF token, using the given client so
// the session cookie it sets is retained for the retried POST. It returns
// ok=false when crumbs are disabled (404) or unreachable — the caller then
// proceeds without, which is correct for a token-authenticated (crumb-exempt)
// instance.
func fetchCrumb(client *http.Client, cfg Config) (field, crumb string, ok bool) {
	resp, err := do(client, cfg, http.MethodGet, "/crumbIssuer/api/json", "", nil)
	if err != nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", "", false
	}
	var out struct {
		Field string `json:"crumbRequestField"`
		Crumb string `json:"crumb"`
	}
	if json.Unmarshal(resp.Body, &out) != nil || out.Crumb == "" {
		return "", "", false
	}
	if out.Field == "" {
		out.Field = "Jenkins-Crumb"
	}
	return out.Field, out.Crumb, true
}

// CheckResponse verifies the status is 2xx or one of the extra codes an action
// declares acceptable (a build trigger answers 201, a restart 503/302). On
// failure it distils a short message from the body — Jenkins error pages are
// often HTML, so tags are stripped and the text is truncated.
func CheckResponse(resp *Response, acceptable ...int) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	for _, code := range acceptable {
		if resp.StatusCode == code {
			return nil
		}
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("Jenkins rejected the request as unauthorised (HTTP %d) — check the Username and API Token", resp.StatusCode)
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("Jenkins returned 404 Not Found — check the Jenkins URL and that the job exists")
	}
	if msg := snippet(resp.Body); msg != "" {
		return fmt.Errorf("Jenkins API error (%d): %s", resp.StatusCode, msg)
	}
	return fmt.Errorf("Jenkins API error (%d)", resp.StatusCode)
}

// snippet reduces an error body — frequently an HTML page — to a short,
// single-line hint.
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

// DecodeObject unmarshals a JSON object response (…/api/json) into a map.
// An empty body yields an empty map.
func DecodeObject(resp *Response) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Jenkins response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// OptionalString extracts a string input, returning "" if absent. The explicit
// Value==nil guard matters for the Code and Secret input types (xml, api_token):
// unlike the String type, their String() renders a nil value as the literal
// "<nil>" rather than "", so an unset field would otherwise read as non-empty.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}

// RequiredString extracts a required string input, erroring if absent/blank.
func RequiredString(name, label string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return v, nil
}

// OptionalInt extracts an integer input. The bool is false when absent, so
// callers distinguish "unset" from "set to 0".
//
// The type switch guards a sharp edge in core Connection.Number(): its final
// fallback is an unchecked c.Value.(string) assertion, which panics when a
// whole-value ${...} reference lands a slice/map/bool in an integer-typed input
// (e.g. a user wires "Limit" to an upstream array). Only values Number() can
// actually handle reach it; anything else reads as unset rather than crashing
// the one-shot executor.
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return 0, false
	}
	switch v := conn.Value.(type) {
	case int, int64, float64:
		// Number() handles these numeric kinds safely.
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
		// Number() ParseInts the string; a non-numeric string returns nil below.
	default:
		return 0, false
	}
	n := conn.Number()
	if n == nil {
		return 0, false
	}
	return int(*n), true
}

// OptionalBool extracts a boolean input, defaulting to false when unset.
func OptionalBool(name string, inputs []*core.Connection) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false
	}
	return *conn.Boolean()
}

// KeyValues reads a key/value-array input (e.g. build parameters) into an
// ordered url.Values, skipping pairs with a blank key.
func KeyValues(name string, inputs []*core.Connection) url.Values {
	out := url.Values{}
	conn := core.FindConnection(name, inputs)
	if conn == nil {
		return out
	}
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		out.Add(k, kv.Value)
	}
	return out
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ResourceResult shapes a single-object response (get job / get build) into the
// standard action output.
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	return map[string]interface{}{
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection into the standard list output: results array,
// count, plus the mandatory summary/success/error triple.
func ListResult(items []interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// SuccessResult is the standard output for an action whose only signal is that
// it worked (trigger, restart, enable, …). extra keys are merged in.
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

// ErrorResult is the standard soft-failure output (returned with a nil error so
// the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}
