// Package zendesk holds the shared HTTP client, auth helpers, and generic
// resource CRUD used by every helpdesk/zendesk/* action.
//
// Zendesk's Support REST API is uniform across resources — tickets, users and
// organizations share the same create/read/update/delete/list shapes under
// /api/v2/{resource}(/{id}).json, each wrapped in a single-key envelope
// ({"ticket": {...}}, {"users": [...]}). That regularity lets the CRUD live
// here once (CreateResource, GetResource, UpdateResource, DeleteResource,
// ListResources) so each action package stays thin: read its inputs, call one
// helper, shape the result.
//
// Auth mirrors n8n's two Zendesk credentials:
//
//   - API token — an agent email plus an API token, sent as HTTP Basic auth
//     with username "{email}/token" and password "{api_token}". This is the
//     modern Support API token credential and the primary path here.
//   - OAuth2 — an optional bearer access token; when supplied it takes
//     precedence and is sent as "Authorization: Bearer {oauth_token}", giving
//     OAuth2 parity without needing the platform's OAuth-provider machinery
//     (the user brings their own token, exactly as Shopify's GetAuth accepts a
//     ready-made access token).
//
// The subdomain forms the base host ({subdomain}.zendesk.com). api_token and
// oauth_token are ConnectionTypeSecret; subdomain and email ConnectionTypeString.
package zendesk

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// maxResponseBody caps the response body to prevent memory exhaustion.
	// List pages of tickets/users can be large, so 8 MB (the airtable/shopify
	// value) rather than the 1 MB used by single-record integrations.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single Zendesk call.
	requestTimeout = 30 * time.Second

	// MaxAllPages bounds a "return all" pagination loop so a large Zendesk
	// account can never spin unbounded requests. On hitting the cap the action
	// returns the outstanding next_page URL so the caller can resume.
	MaxAllPages = 100

	// DefaultPageLimit / MaxPageLimit bound Zendesk's per_page (1-100).
	DefaultPageLimit = 100
	MaxPageLimit     = 100
)

// httpClient is shared across every Zendesk action so TCP connections to the
// account's API are pooled and reused rather than re-dialled per call (a flow
// run — or a return-all loop — can fire many requests). Matches the
// connection-reuse pattern used by the Shopify and Airtable integrations.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AuthInputs is the shared credential shape. Action packages re-declare their
// own literal Inputs arrays (the manifest generator AST-parses those), but
// this documents the canonical quartet every action puts first.
var AuthInputs = []core.Connection{
	{
		Name:        "subdomain",
		Type:        core.ConnectionTypeString,
		Label:       "Subdomain",
		Placeholder: "mycompany (from mycompany.zendesk.com)",
		Required:    true,
	},
	{
		Name:        "email",
		Type:        core.ConnectionTypeString,
		Label:       "Agent Email",
		Placeholder: "you@company.com (paired with the API token)",
	},
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Token",
		Placeholder: "Zendesk Admin ▸ Apps and integrations ▸ APIs ▸ Zendesk API token",
	},
	{
		Name:        "oauth_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "OAuth Access Token",
		Placeholder: "Optional — a bearer token used instead of the email + API token",
	},
}

// APIResponse wraps the HTTP response for consistent handling. Headers is
// carried so callers can read Retry-After on a 429.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// validSubdomain matches a Zendesk account handle: letters, numbers and
// hyphens only. GetAuth enforces this so a crafted subdomain value can never
// redirect the credentials off zendesk.com.
var validSubdomain = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

// NormaliseSubdomain reduces whatever the user pasted to the bare account
// handle. Accepts "mycompany", "mycompany.zendesk.com", a full
// "https://mycompany.zendesk.com" URL, or an agent URL like
// "https://mycompany.zendesk.com/agent/tickets/1", and returns "mycompany".
// The result is charset-validated by GetAuth before use.
func NormaliseSubdomain(sub string) string {
	sub = strings.TrimSpace(sub)
	sub = strings.TrimPrefix(sub, "https://")
	sub = strings.TrimPrefix(sub, "http://")
	sub = strings.TrimRight(sub, "/")
	sub = strings.TrimSuffix(sub, ".zendesk.com")
	// Drop anything from the first host-significant character onward so a
	// pasted path/port/userinfo can't leak into the assembled host.
	if i := strings.IndexAny(sub, "/.?#:@"); i >= 0 {
		sub = sub[:i]
	}
	return sub
}

// hostForSubdomain returns the scheme+host for an account's API. It is a var
// rather than inline so tests can point every request at an httptest server
// (the same seam idiom as Shopify's hostForShop).
var hostForSubdomain = func(sub string) string {
	return fmt.Sprintf("https://%s.zendesk.com", sub)
}

// BuildURL assembles a full API URL for a resource path (which must start with
// "/", e.g. "/tickets.json").
func BuildURL(sub, path string) string {
	return hostForSubdomain(sub) + "/api/v2" + path
}

// SetHostForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores the real host. It lets action
// packages in sibling directories exercise Execute end-to-end without hitting
// Zendesk. Test-only.
func SetHostForTest(base string) func() {
	prev := hostForSubdomain
	hostForSubdomain = func(string) string { return base }
	return func() { hostForSubdomain = prev }
}

// GetAuth resolves the account subdomain and an Authorization header value from
// the action's auth inputs. Two modes are supported, in priority order:
//
//  1. OAuth bearer — an oauth_token supplied directly ("Bearer {token}").
//  2. API token — an agent email + API token, HTTP Basic-encoded as
//     "{email}/token:{api_token}".
//
// A missing subdomain, or neither credential form, is a hard failure (empty
// result + real error) rather than a soft error output.
func GetAuth(inputs []*core.Connection) (subdomain, authHeader string, err error) {
	subRaw, err := RequiredString("subdomain", inputs)
	if err != nil {
		return "", "", err
	}
	subdomain = NormaliseSubdomain(subRaw)
	if !validSubdomain.MatchString(subdomain) {
		return "", "", fmt.Errorf("subdomain must be your Zendesk account handle (letters, numbers, hyphens) — e.g. mycompany from mycompany.zendesk.com")
	}

	// A directly-supplied OAuth token wins — no email needed.
	if tok := OptionalString("oauth_token", inputs); tok != "" {
		return subdomain, "Bearer " + tok, nil
	}

	email := OptionalString("email", inputs)
	apiToken := OptionalString("api_token", inputs)
	if email != "" && apiToken != "" {
		creds := base64.StdEncoding.EncodeToString([]byte(email + "/token:" + apiToken))
		return subdomain, "Basic " + creds, nil
	}

	return "", "", fmt.Errorf("authentication required: provide an OAuth Access Token, or an Agent Email and API Token")
}

// ExecuteAPI performs a REST call to the account's Support API.
// method: GET, POST, PUT, DELETE
// path:   resource path including any query string (e.g. "/tickets/1.json?...")
// body:   optional payload — marshalled to JSON for POST/PUT, ignored otherwise
func ExecuteAPI(subdomain, authHeader, method, path string, body interface{}) (*APIResponse, error) {
	return executeAbsolute(authHeader, method, BuildURL(subdomain, path), body)
}

// executeAbsolute issues a request to a fully-qualified URL. List pagination
// follows Zendesk's absolute next_page URLs, so those go through here directly
// rather than being re-assembled from a subdomain + path.
func executeAbsolute(authHeader, method, fullURL string, body interface{}) (*APIResponse, error) {
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
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", authHeader)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Zendesk API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// CheckResponse verifies a 2xx status, decoding Zendesk's error envelope. The
// "error" field may be a plain string ("RecordNotFound") or an object
// ({"title": "...", "message": "..."}); "description" carries a human message
// and "details" holds per-field validation errors, so all are surfaced. 429 is
// reported with its Retry-After so the caller understands it hit the rate limit.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		if retry := resp.Headers.Get("Retry-After"); retry != "" {
			return fmt.Errorf("Zendesk API rate limit exceeded (429); retry after %ss", retry)
		}
		return fmt.Errorf("Zendesk API rate limit exceeded (429)")
	}

	if msg := formatZendeskError(resp.Body); msg != "" {
		return fmt.Errorf("Zendesk API error (%d): %s", resp.StatusCode, msg)
	}
	return fmt.Errorf("Zendesk API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// formatZendeskError extracts a readable message from a Zendesk error body,
// combining the top-level error/description with any per-field validation
// details. Returns "" when the body isn't a recognisable error envelope.
func formatZendeskError(body []byte) string {
	var env struct {
		Error       interface{}            `json:"error"`
		Description string                 `json:"description"`
		Details     map[string]interface{} `json:"details"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return ""
	}

	var parts []string
	switch e := env.Error.(type) {
	case string:
		if e != "" {
			parts = append(parts, e)
		}
	case map[string]interface{}:
		if title, _ := e["title"].(string); title != "" {
			parts = append(parts, title)
		}
		if message, _ := e["message"].(string); message != "" {
			parts = append(parts, message)
		}
	}
	if env.Description != "" {
		parts = append(parts, env.Description)
	}
	if len(env.Details) > 0 {
		if b, err := json.Marshal(env.Details); err == nil {
			parts = append(parts, string(b))
		}
	}
	return strings.Join(parts, ": ")
}

// DecodeBody is the exported form of decode, for the few actions (e.g. ticket
// recover, organization count) that issue a request outside the generic CRUD
// helpers and need to unmarshal the raw success body themselves.
func DecodeBody(resp *APIResponse) (map[string]interface{}, error) {
	return decode(resp)
}

// decode unmarshals a successful response body into a generic map. An empty
// body (e.g. a delete's 204) yields an empty map.
func decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Zendesk response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Input helpers (mirrors of the Shopify shapes)
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

// OptionalInt extracts an integer input. The bool is false when absent, so
// callers distinguish "unset" from "set to 0".
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
// Used for the nested structures Zendesk takes (custom_fields, user_fields,
// organization_fields, and the additional_fields escape hatch). Returns
// (nil, nil) when the input is absent/blank, (nil, err) on malformed JSON so
// the action can surface a clear message.
func OptionalJSON(name string, inputs []*core.Connection) (interface{}, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil {
		return nil, nil
	}
	// Object/array inputs may arrive already-parsed (from ${...} wiring) or as
	// a JSON string typed into the editor.
	if conn.Value != nil {
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
	return nil, nil
}

// SplitCSV turns a comma-separated string ("vip, wholesale") into a trimmed,
// non-empty slice (["vip","wholesale"]) — used for tags and domain_names,
// which Zendesk takes as arrays. Returns nil for an empty input.
func SplitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SetIfPresent adds an optional string field to a resource body only when the
// input was provided, so unset fields are omitted.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// SetCSVIfPresent adds an optional comma-separated input as a string array
// field only when non-empty (tags, domain_names).
func SetCSVIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := SplitCSV(OptionalString(inputName, inputs)); v != nil {
		body[field] = v
	}
}

// SetIntIfSet adds an optional integer field when its input is present, so the
// unset case is omitted rather than sent as 0.
func SetIntIfSet(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v, set := OptionalInt(inputName, inputs); set {
		body[field] = v
	}
}

// SetNumericIDIfPresent adds an optional ID field, sent as a number when the
// value parses as an integer (Zendesk's group_id/assignee_id/organization_id
// etc. are numeric, and dropdown/entered values arrive as strings). A
// non-numeric value is passed through as a string rather than dropped, so an
// unexpected format still reaches Zendesk to surface its own error.
func SetNumericIDIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	v := OptionalString(inputName, inputs)
	if v == "" {
		return
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		body[field] = n
		return
	}
	body[field] = v
}

// SetBoolIfSet adds an optional boolean field when its input connection is
// present (checkbox touched), so the tri-state nil is preserved as "omit".
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

// MergeAdditionalFields overlays a raw JSON object input ("additional_fields")
// onto the resource body, the escape hatch for any Zendesk field not exposed as
// a first-class input. Later keys win. Returns an error on malformed JSON or a
// non-object payload.
func MergeAdditionalFields(body map[string]interface{}, inputs []*core.Connection) error {
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
		body[k] = val
	}
	return nil
}

// AddFilter sets a query param from an optional string input when non-empty.
func AddFilter(q url.Values, inputs []*core.Connection, param, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		q.Set(param, v)
	}
}

// ClampLimit bounds a requested per_page to Zendesk's 1-100 range, falling back
// to DefaultPageLimit when unset.
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
// Generic resource CRUD
// ---------------------------------------------------------------------------

// CreateResource POSTs a new resource. resourcePath is like "/tickets.json";
// resourceKey ("ticket"/"user"/"organization") wraps the body and unwraps the
// response.
func CreateResource(subdomain, auth, resourcePath, resourceKey string, fields map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{resourceKey: fields}
	resp, err := ExecuteAPI(subdomain, auth, http.MethodPost, resourcePath, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// GetResource GETs a single resource by path (e.g. "/tickets/1.json"), applying
// optional query params.
func GetResource(subdomain, auth, path string, q url.Values) (map[string]interface{}, error) {
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	resp, err := ExecuteAPI(subdomain, auth, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// UpdateResource PUTs changes to a single resource by path.
func UpdateResource(subdomain, auth, path, resourceKey string, fields map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{resourceKey: fields}
	resp, err := ExecuteAPI(subdomain, auth, http.MethodPut, path, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// DeleteResource DELETEs a single resource by path. Zendesk returns 200/204 on
// success.
func DeleteResource(subdomain, auth, path string) error {
	resp, err := ExecuteAPI(subdomain, auth, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ListResources fetches a collection. propertyName is the plural envelope key
// ("tickets"/"users"/"organizations"/"results"/"suspended_tickets"). When
// returnAll is false a single page is fetched and the next_page URL (if any) is
// returned so the caller can resume manually. When true it follows Zendesk's
// absolute next_page URL until exhausted or the MaxAllPages cap. Returns the
// accumulated items, the outstanding next_page URL (empty when fully drained),
// the last raw response, and the number of pages fetched.
func ListResources(subdomain, auth, resourcePath, propertyName string, q url.Values, returnAll bool) (items []interface{}, nextPage string, lastRaw map[string]interface{}, pages int, err error) {
	// Non-nil so a zero-match list serialises as [] not null — get-many is
	// consumed by Loop nodes that iterate the array.
	items = []interface{}{}

	path := resourcePath
	if q != nil {
		if enc := q.Encode(); enc != "" {
			path += "?" + enc
		}
	}

	// First page goes through the subdomain+path builder; subsequent pages
	// follow Zendesk's absolute next_page URLs directly.
	resp, e := ExecuteAPI(subdomain, auth, http.MethodGet, path, nil)
	for {
		if e != nil {
			return nil, "", nil, pages, e
		}
		if e := CheckResponse(resp); e != nil {
			return nil, "", nil, pages, e
		}
		raw, e := decode(resp)
		if e != nil {
			return nil, "", nil, pages, e
		}
		lastRaw = raw
		pages++
		if arr, ok := raw[propertyName].([]interface{}); ok {
			items = append(items, arr...)
		}

		nextPage, _ = raw["next_page"].(string)
		if !returnAll || nextPage == "" || pages >= MaxAllPages {
			break
		}
		resp, e = executeAbsolute(auth, http.MethodGet, nextPage, nil)
	}
	return items, nextPage, lastRaw, pages, nil
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ErrorResult is the standard soft-failure output map (returned with a nil
// error so the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ResourceResult shapes a single-resource response (create/get/update) into the
// standard action output. resourceKey unwraps Zendesk's {"ticket": {...}}
// envelope; id is stringified from the unwrapped object.
func ResourceResult(resp map[string]interface{}, resourceKey, summary string) map[string]interface{} {
	obj, _ := resp[resourceKey].(map[string]interface{})
	if obj == nil {
		obj = resp
	}
	return map[string]interface{}{
		"id":          StringifyID(obj["id"]),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output:
// results array, count, the outstanding next_page cursor, the raw last page,
// plus the mandatory summary/success/error triple.
func ListResult(items []interface{}, nextPage string, lastRaw map[string]interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"next_page":   nextPage,
		"result":      lastRaw,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// StringifyID renders Zendesk's numeric IDs (which decode to float64 from JSON)
// as a clean integer string, leaving string IDs untouched.
func StringifyID(id interface{}) string {
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
