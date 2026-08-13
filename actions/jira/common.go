// Package jira_common holds the shared HTTP client, auth helpers, and generic
// resource CRUD used by every jira/* action.
//
// Jira Cloud's REST API is broad but regular: issues, comments, attachments,
// users and worklogs all sit under {site}/rest/api/2/... with conventional
// create/read/update/delete/list shapes. That regularity lets the transport,
// auth, error handling, pagination and result-shaping live here once, so each
// action package stays thin — the same design as the sibling cms/wordpress and
// ecommerce/woocommerce packages.
//
// Four things shape this file:
//
//   - Auth is HTTP Basic — an Atlassian account email plus an API token
//     (id.atlassian.com ▸ Security ▸ API tokens). The token is a
//     ConnectionTypeSecret; the site URL and email are ConnectionTypeString.
//
//   - We pin API v2 (not v3). On Jira Cloud, v3 requires the Atlassian Document
//     Format (ADF) for rich-text fields (issue description, comment/worklog
//     body) — a nested JSON document that a non-technical operator can't be
//     expected to hand-author. v2 accepts and renders plain strings for those
//     same fields, which is the operator-simple default. v2 is fully supported
//     on Cloud. (n8n makes the same choice for the Issue resource.)
//
//   - Responses vary: create/get return the bare object; update/delete return
//     204 No Content (synthesised into a success result); list operations wrap
//     items in an envelope under a named property (values / comments / worklogs)
//     with startAt / maxResults / total for offset pagination; the issue search
//     endpoint (/search/jql) uses opaque nextPageToken cursor pagination.
//
//   - Errors come back as {"errorMessages":[...],"errors":{field:msg}} — both
//     are surfaced, and the API token is scrubbed from any error string.
package jira_common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// APIBasePath is the Jira REST API v2 prefix appended to the site URL. See
	// the package doc for why v2 (not v3) — plain-text rich fields for operators.
	APIBasePath = "/rest/api/2"

	// maxResponseBody caps the response body to prevent memory exhaustion. Issue
	// search with many fields can be large, so 8 MB.
	maxResponseBody = 8 << 20 // 8 MB

	// MaxAttachmentBytes bounds attachment upload/download so a runaway file
	// can't exhaust memory. Jira Cloud's own default attachment ceiling is
	// smaller, so this is a generous safety cap, not the functional limit. Both
	// the upload (decoded base64) and the download (streamed bytes) enforce it,
	// and the download errors rather than silently truncating past it.
	MaxAttachmentBytes = 50 << 20 // 50 MB

	// requestTimeout is the HTTP client timeout for a single Jira call.
	requestTimeout = 30 * time.Second

	// MaxAllPages bounds a "return all" pagination loop so a huge backlog can
	// never spin unbounded requests. On hitting the cap the action reports it.
	MaxAllPages = 100

	// DefaultPageLimit / MaxPageLimit bound a single page. Jira caps most list
	// endpoints at 100 per page; issue search allows up to 100 via the UI too.
	DefaultPageLimit = 50
	MaxPageLimit     = 100
)

// httpClient is shared across every Jira action so TLS connections to the site
// are pooled and reused rather than re-dialled per call.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AuthInputs is the shared credential shape. Action packages re-declare their
// own literal Inputs arrays (the manifest generator AST-parses those), but this
// documents the canonical set every action puts first.
//
// The secret is named api_token (not "token"/"password") so a resource field
// never collides with it: core.FindConnection returns the FIRST input whose name
// matches, so a resource field sharing a credential's name would silently
// resolve to the credential. Likewise the auth email is "email"; the user
// resource's own email field is "email_address" to stay clear of it.
var AuthInputs = []core.Connection{
	{
		Name:        "url",
		Type:        core.ConnectionTypeString,
		Label:       "Site URL",
		Placeholder: "https://your-domain.atlassian.net",
		Required:    true,
	},
	{
		Name:        "email",
		Type:        core.ConnectionTypeString,
		Label:       "Account Email",
		Placeholder: "The Atlassian account email that owns the API token",
		Required:    true,
	},
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Token",
		Placeholder: "id.atlassian.com ▸ Security ▸ Create and manage API tokens",
		Required:    true,
	},
}

// Auth is the resolved connection: a normalised base URL (scheme + host, no
// trailing slash, no /rest suffix), the account email and the API token.
type Auth struct {
	BaseURL string
	Email   string
	Token   string
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ---------------------------------------------------------------------------
// Auth resolution
// ---------------------------------------------------------------------------

// GetAuth resolves the site URL, email and API token from the action's auth
// inputs. A missing part is a hard failure — there is nothing to attempt.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	rawURL, err := RequiredString("url", inputs)
	if err != nil {
		return Auth{}, err
	}
	base, err := NormaliseBaseURL(rawURL)
	if err != nil {
		return Auth{}, err
	}
	email, err := RequiredString("email", inputs)
	if err != nil {
		return Auth{}, err
	}
	token, err := RequiredString("api_token", inputs)
	if err != nil {
		return Auth{}, err
	}
	return Auth{BaseURL: base, Email: email, Token: token}, nil
}

// NormaliseBaseURL reduces whatever the user pasted to a clean scheme+host base
// with no trailing slash and no REST-API suffix, defaulting to https.
func NormaliseBaseURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("url is required")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("url is not a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("url must be an http(s) URL, e.g. https://your-domain.atlassian.net")
	}
	if u.Host == "" {
		return "", fmt.Errorf("url must include a host, e.g. https://your-domain.atlassian.net")
	}
	path := strings.TrimRight(u.Path, "/")
	// Strip a pasted /rest or /rest/api/N suffix so we always own the API prefix.
	// Kept in sync with the api option-proxy and launch trigger normalisers.
	for _, suffix := range []string{"/rest/api/3", "/rest/api/2", "/rest/api/latest", "/rest"} {
		if strings.HasSuffix(path, suffix) {
			path = strings.TrimSuffix(path, suffix)
			break
		}
	}
	u.User = nil
	return u.Scheme + "://" + u.Host + path, nil
}

// apiBase assembles the REST API root for a site. It is a var (not inline) so
// SetBaseForTest can point every request at an httptest server.
var apiBase = func(a Auth) string {
	return strings.TrimRight(a.BaseURL, "/") + APIBasePath
}

// SetBaseForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores the real base. Test-only.
func SetBaseForTest(base string) func() {
	prev := apiBase
	apiBase = func(Auth) string { return strings.TrimRight(base, "/") }
	return func() { apiBase = prev }
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// ExecuteAPI performs a REST call to the site's Jira API.
// method: GET, POST, PUT, DELETE (Jira uses PUT for updates).
// path:   resource path under the API base including any query string
//         (e.g. "/issue/SCRUM-1?fields=summary").
// body:   optional payload — marshalled to JSON for POST/PUT, ignored otherwise.
func ExecuteAPI(a Auth, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := apiBase(a) + path

	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPut) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", redactAuth(a, err.Error()))
	}

	req.SetBasicAuth(a.Email, a.Token)
	req.Header.Set("Accept", "application/json")
	// Required by Jira for attachment upload XSRF; harmless on every other call.
	req.Header.Set("X-Atlassian-Token", "no-check")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Jira API request failed: %s", redactAuth(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// DoRaw performs a request with an arbitrary Content-Type and raw body reader —
// used by the attachment upload (multipart/form-data), which cannot go through
// the JSON path. The caller supplies the full body and content type.
func DoRaw(a Auth, method, path, contentType string, body io.Reader) (*APIResponse, error) {
	fullURL := apiBase(a) + path
	req, err := http.NewRequest(method, fullURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", redactAuth(a, err.Error()))
	}
	req.SetBasicAuth(a.Email, a.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Atlassian-Token", "no-check")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Jira API request failed: %s", redactAuth(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// redactAuth removes the API token from an error message. It travels in the
// Basic auth header (not the URL), so a leak is unlikely — but a proxy or
// wrapped error could echo it, so it is scrubbed defensively.
func redactAuth(a Auth, msg string) string {
	if a.Token != "" {
		msg = strings.ReplaceAll(msg, a.Token, "REDACTED")
	}
	return msg
}

// CheckResponse verifies a 2xx status, decoding Jira's error envelope
// ({"errorMessages":[...],"errors":{field:reason}}). Both the top-level messages
// and the per-field errors are surfaced so a validation failure names the field.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var env struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
	}
	parts := []string{}
	if err := json.Unmarshal(resp.Body, &env); err == nil {
		parts = append(parts, env.ErrorMessages...)
		for field, reason := range env.Errors {
			parts = append(parts, fmt.Sprintf("%s: %s", field, reason))
		}
	}
	if len(parts) > 0 {
		return fmt.Errorf("Jira API error (%d): %s", resp.StatusCode, strings.Join(parts, "; "))
	}
	body := string(resp.Body)
	if len(body) > 500 {
		body = body[:500]
	}
	if strings.TrimSpace(body) == "" {
		body = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("Jira API error (%d): %s", resp.StatusCode, body)
}

// decode unmarshals a successful single-resource body into a generic map. An
// empty body (e.g. a 204) yields an empty map.
func decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Jira response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// OptionalString extracts a string input, returning "" if absent.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}

// RequiredString extracts a required string input, erroring if absent/blank.
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
	if conn == nil || conn.Number() == nil {
		return 0, false
	}
	return int(*conn.Number()), true
}

// OptionalBool extracts a boolean input, defaulting to false when unset.
func OptionalBool(name string, inputs []*core.Connection) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false
	}
	return *conn.Boolean()
}

// OptionalJSON parses an object/array-typed input into an arbitrary value.
// Returns (nil, nil) when absent/blank, (nil, err) on malformed JSON.
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

// SetIfPresent adds an optional string field to a body only when provided.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// SetIntIfPresent parses an optional string/integer input as an integer and adds
// it to the body when present. A non-numeric value is surfaced as an error.
func SetIntIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) error {
	if n, ok := OptionalInt(inputName, inputs); ok {
		body[field] = n
		return nil
	}
	v := OptionalString(inputName, inputs)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("%s must be a whole number", inputName)
	}
	body[field] = n
	return nil
}

// SetBoolIfSet adds an optional boolean field when its input connection is
// present (checkbox touched), preserving the tri-state nil as "omit".
func SetBoolIfSet(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	conn := core.FindConnection(inputName, inputs)
	if conn != nil && conn.Boolean() != nil {
		body[field] = *conn.Boolean()
	}
}

// SetJSONIfPresent parses an optional JSON input and adds it to the body when
// present. Returns an error only on malformed JSON.
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

// StringList splits a comma-separated input into a trimmed []string, dropping
// blanks. Used for labels, component ids, etc. Editor multi-value inputs are
// single-select, so multi-value fields are entered as comma-separated text.
func StringList(name string, inputs []*core.Connection) []string {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// MergeAdditionalFields overlays a raw JSON object input ("additional_fields")
// onto a target map — the escape hatch for any field not exposed as a
// first-class input. Called LAST in body assembly, so a key here OVERRIDES the
// same key set by a first-class input (the "power-user last word" precedence
// shared with the WordPress / WooCommerce nodes).
func MergeAdditionalFields(target map[string]interface{}, inputs []*core.Connection) error {
	v, err := OptionalJSON("additional_fields", inputs)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Errorf(`additional_fields must be a JSON object, e.g. {"key":"value"}`)
	}
	for k, val := range obj {
		target[k] = val
	}
	return nil
}

// ClampLimit bounds a requested page size to Jira's 1-100 range, falling back to
// DefaultPageLimit when unset.
func ClampLimit(limit int, set bool) int {
	if !set || limit <= 0 {
		return DefaultPageLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit
	}
	return limit
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// summaryWithData folds the structured payload into the tool_result string so an
// AI caller gets both the human summary and the underlying data. The engine's
// tool-result fallback chain uses tool_result VERBATIM when non-empty and never
// falls through to result/results, so a bare summary would starve the AI of the
// actual object/list. Marshalling failures (or a null/empty payload) degrade to
// the summary alone.
func summaryWithData(summary string, data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil || len(b) == 0 || string(b) == "null" {
		return summary
	}
	if summary == "" {
		return string(b)
	}
	return summary + "\n" + string(b)
}

// ErrorResult is the standard soft-failure output map (returned with a nil error
// so the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ResourceResult shapes a single-resource response (create/get) into the
// standard action output. The id output prefers a human-facing issue "key"
// (e.g. SCRUM-1) when present, else the numeric "id".
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	id := stringifyID(obj["key"])
	if id == "" {
		id = stringifyID(obj["id"])
	}
	return map[string]interface{}{
		"id":          id,
		"result":      obj,
		"tool_result": summaryWithData(summary, obj),
		"success":     true,
		"error":       "",
	}
}

// SuccessResult shapes an operation that returns no body (204 update/delete) or
// whose id the caller already knows. result carries any small confirmation map.
func SuccessResult(id string, result map[string]interface{}, summary string) map[string]interface{} {
	if result == nil {
		result = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          id,
		"result":      result,
		"tool_result": summaryWithData(summary, result),
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output.
func ListResult(items []interface{}, total int, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"total":       total,
		"tool_result": summaryWithData(summary, items),
		"success":     true,
		"error":       "",
	}
}

// stringifyID renders Jira ids (numeric ids decode to float64 from JSON, keys
// are strings) as a clean string.
func stringifyID(id interface{}) string {
	switch v := id.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}

// ---------------------------------------------------------------------------
// Generic resource CRUD
// ---------------------------------------------------------------------------

// CreateResource POSTs a new resource and decodes the bare returned object.
func CreateResource(a Auth, path string, fields map[string]interface{}) (map[string]interface{}, error) {
	resp, err := ExecuteAPI(a, http.MethodPost, path, fields)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// GetResource GETs a single resource by path, applying optional query params.
func GetResource(a Auth, path string, q url.Values) (map[string]interface{}, error) {
	if q != nil {
		if enc := q.Encode(); enc != "" {
			path += "?" + enc
		}
	}
	resp, err := ExecuteAPI(a, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// UpdateResource PUTs changes to a single resource. Jira returns 204 No Content
// on a successful update, so there is no body to decode — success is the 2xx.
func UpdateResource(a Auth, path string, fields map[string]interface{}) error {
	resp, err := ExecuteAPI(a, http.MethodPut, path, fields)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// PutResource PUTs changes and decodes the returned object. Some Jira PUTs
// (comment, worklog) respond 200 with the updated resource, unlike issue update
// which responds 204 (use UpdateResource for those).
func PutResource(a Auth, path string, fields map[string]interface{}) (map[string]interface{}, error) {
	resp, err := ExecuteAPI(a, http.MethodPut, path, fields)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// GetBinary fetches raw bytes from an absolute Jira URL (e.g. an attachment's
// "content" link, which comes back as a fully-qualified URL, not an API path).
// Auth and the XSRF header are applied. The download is bounded by
// MaxAttachmentBytes; a file that exceeds it is rejected with an error rather
// than silently truncated (returning a corrupt partial file would be worse).
func GetBinary(a Auth, absoluteURL string) ([]byte, string, error) {
	req, err := http.NewRequest(http.MethodGet, absoluteURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create request: %s", redactAuth(a, err.Error()))
	}
	req.SetBasicAuth(a.Email, a.Token)
	req.Header.Set("X-Atlassian-Token", "no-check")
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("Jira attachment download failed: %s", redactAuth(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, "", fmt.Errorf("Jira attachment download error (%d)", resp.StatusCode)
	}
	// Read one byte past the cap so an over-size file is detected, not truncated.
	data, err := io.ReadAll(io.LimitReader(resp.Body, MaxAttachmentBytes+1))
	if err != nil {
		return nil, "", fmt.Errorf("failed to read attachment: %w", err)
	}
	if len(data) > MaxAttachmentBytes {
		return nil, "", fmt.Errorf("attachment exceeds the %d MB download limit", MaxAttachmentBytes>>20)
	}
	return data, resp.Header.Get("Content-Type"), nil
}

// PostNoContent POSTs a body to an endpoint that returns 204 (transitions,
// notify, worklog-less side calls). Success is the 2xx.
func PostNoContent(a Auth, path string, body interface{}) error {
	resp, err := ExecuteAPI(a, http.MethodPost, path, body)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// DeleteResource DELETEs a single resource by path, applying optional query
// params (e.g. deleteSubtasks, reassign). Jira returns 204 on success.
func DeleteResource(a Auth, path string, q url.Values) error {
	if q != nil {
		if enc := q.Encode(); enc != "" {
			path += "?" + enc
		}
	}
	resp, err := ExecuteAPI(a, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

// ListOffset fetches a collection that paginates with startAt / maxResults and
// wraps its items under `property` (e.g. "values", "comments", "worklogs"). When
// returnAll is false a single page (from startAt, size limit) is returned. When
// true it walks pages until `isLast` is true or the accumulated count reaches
// `total`, up to the MaxAllPages cap. Extra query params (order, expand) come in
// via q. Returns the items and the reported total.
func ListOffset(a Auth, path, property string, q url.Values, startAt, limit int, returnAll bool) ([]interface{}, int, error) {
	items := []interface{}{}
	if q == nil {
		q = url.Values{}
	}
	pageSize := ClampLimit(limit, limit > 0)
	total := 0
	pages := 0

	for {
		q.Set("startAt", strconv.Itoa(startAt))
		q.Set("maxResults", strconv.Itoa(pageSize))
		resp, err := ExecuteAPI(a, http.MethodGet, path+"?"+q.Encode(), nil)
		if err != nil {
			return nil, 0, err
		}
		if err := CheckResponse(resp); err != nil {
			return nil, 0, err
		}
		var env map[string]json.RawMessage
		if err := json.Unmarshal(resp.Body, &env); err != nil {
			return nil, 0, fmt.Errorf("failed to parse Jira response: %w", err)
		}
		var page []interface{}
		if raw, ok := env[property]; ok {
			if err := json.Unmarshal(raw, &page); err != nil {
				return nil, 0, fmt.Errorf("failed to parse Jira %s list: %w", property, err)
			}
		}
		items = append(items, page...)
		pages++

		if raw, ok := env["total"]; ok {
			var t int
			if json.Unmarshal(raw, &t) == nil {
				total = t
			}
		}
		isLast := false
		if raw, ok := env["isLast"]; ok {
			_ = json.Unmarshal(raw, &isLast)
		}

		if !returnAll || len(page) == 0 || pages >= MaxAllPages {
			break
		}
		startAt += len(page)
		// Stop when the envelope says this was the last page, or when we've
		// gathered everything the total advertises.
		if isLast || (total > 0 && startAt >= total) {
			break
		}
	}
	if total == 0 {
		total = len(items)
	}
	return items, total, nil
}

// SearchJQL runs an issue search against the Cloud /search/jql endpoint, which
// paginates with an opaque nextPageToken cursor (the classic /search endpoint
// was removed on Cloud). fields is the comma-separated field list to return
// (defaults to *navigable). When returnAll is false a single page (size limit)
// is returned; when true the cursor is followed to the MaxAllPages cap.
func SearchJQL(a Auth, jql, fields string, expand []string, limit int, returnAll bool) ([]interface{}, error) {
	issues := []interface{}{}
	pageSize := ClampLimit(limit, limit > 0)
	if strings.TrimSpace(jql) == "" {
		// Jira rejects an unbounded search; a always-true bound matches n8n.
		jql = `created >= "1970-01-01"`
	}
	fieldList := []string{"*navigable"}
	if strings.TrimSpace(fields) != "" {
		fieldList = []string{}
		for _, f := range strings.Split(fields, ",") {
			if p := strings.TrimSpace(f); p != "" {
				fieldList = append(fieldList, p)
			}
		}
	}
	nextToken := ""
	pages := 0
	for {
		body := map[string]interface{}{
			"jql":        jql,
			"fields":     fieldList,
			"maxResults": pageSize,
		}
		if len(expand) > 0 {
			body["expand"] = expand
		}
		if nextToken != "" {
			body["nextPageToken"] = nextToken
		}
		resp, err := ExecuteAPI(a, http.MethodPost, "/search/jql", body)
		if err != nil {
			return nil, err
		}
		if err := CheckResponse(resp); err != nil {
			return nil, err
		}
		var env struct {
			Issues        []interface{} `json:"issues"`
			NextPageToken string        `json:"nextPageToken"`
			IsLast        bool          `json:"isLast"`
		}
		if err := json.Unmarshal(resp.Body, &env); err != nil {
			return nil, fmt.Errorf("failed to parse Jira search response: %w", err)
		}
		issues = append(issues, env.Issues...)
		pages++
		if !returnAll || env.NextPageToken == "" || env.IsLast || len(env.Issues) == 0 || pages >= MaxAllPages {
			break
		}
		nextToken = env.NextPageToken
	}
	return issues, nil
}
