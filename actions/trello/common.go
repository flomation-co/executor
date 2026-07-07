// Package trello_common holds the shared HTTP client, auth helpers, and generic
// resource helpers used by every trello/* action.
//
// Trello's REST API is small and regular. Everything hangs off a single base —
// https://api.trello.com/1 — and every resource (boards, lists, cards,
// checklists, labels, attachments, members) follows conventional
// create/read/update/delete/list shapes. That regularity lets the transport,
// auth, error handling and result-shaping live here once, so each action package
// stays thin — the same design as the sibling jira / cms.wordpress packages.
//
// Four things shape this file:
//
//   - Auth is an API key + an API token, both travelling as query parameters
//     (?key=…&token=…). This is Trello's documented scheme — there is no OAuth
//     dance for a personal token. Both are ConnectionTypeSecret so they render as
//     environment-secret pickers (the operator selects ${secrets.Trello_API} /
//     ${secrets.Trello_token}); the plaintext is scrubbed from any error string.
//
//   - The base host is FIXED (api.trello.com) — it is never caller-supplied — so
//     there is no SSRF surface here and no self-signed/insecure path to gate
//     (contrast the self-hosted WordPress/Jira nodes). The URL is assembled from
//     constants, so a crafted input can't redirect the request to another host.
//
//   - Writes go through the query string too. Trello accepts create/update
//     parameters as either a JSON body or query parameters; using the query
//     string uniformly (for POST/PUT as well as GET/DELETE) gives one clean code
//     path and matches how n8n's Trello node issues its update calls. Values are
//     URL-encoded via url.Values, so text with spaces/newlines is safe.
//
//   - Responses are bare — a single object ({...}) for get/create/update, a JSON
//     array ([...]) for list operations, and either a small object or the deleted
//     resource for delete. There is no envelope, so the shapers below decode the
//     body generically and read "id" for the id output.
package trello_common

import (
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
	// APIBase is the fixed Trello REST API root. It is a constant (never
	// caller-supplied), so every request targets api.trello.com and there is no
	// SSRF surface — a crafted input cannot point the client at another host.
	APIBase = "https://api.trello.com/1"

	// maxResponseBody caps the response body to prevent memory exhaustion. A
	// board with many cards/actions can be large, so 8 MB.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single Trello call.
	requestTimeout = 30 * time.Second

	// DefaultPageLimit / MaxPageLimit bound a single list page. Trello caps most
	// list endpoints at 1000; a dropdown/flow rarely wants more than a page.
	DefaultPageLimit = 100
	MaxPageLimit     = 1000
)

// httpClient is shared across every Trello action so TLS connections to
// api.trello.com are pooled and reused rather than re-dialled per call.
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
// The secrets are named api_key / api_token (not "key"/"token") so a resource
// field never collides with them: core.FindConnection returns the FIRST input
// whose name matches, so a resource field sharing a credential's name would
// silently resolve to the credential. No Trello resource field uses those names.
var AuthInputs = []core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Key",
		Placeholder: "Your Trello API key — trello.com/power-ups/admin ▸ your Power-Up ▸ API Key",
		Required:    true,
	},
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Token",
		Placeholder: "Your Trello API token — generated from the same API Key page",
		Required:    true,
	},
}

// Auth is the resolved credential: the API key and token that authenticate every
// call as query parameters.
type Auth struct {
	Key   string
	Token string
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

// GetAuth resolves the API key and token from the action's auth inputs. A
// missing part is a hard failure — there is nothing to attempt.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	key, err := RequiredString("api_key", inputs)
	if err != nil {
		return Auth{}, err
	}
	token, err := RequiredString("api_token", inputs)
	if err != nil {
		return Auth{}, err
	}
	return Auth{Key: key, Token: token}, nil
}

// apiBase is a var (not a const use) so SetBaseForTest can point every request
// at an httptest server.
var apiBase = APIBase

// SetBaseForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores the real base. Test-only.
func SetBaseForTest(base string) func() {
	prev := apiBase
	apiBase = strings.TrimRight(base, "/")
	return func() { apiBase = prev }
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// Do performs a REST call to Trello. method is GET/POST/PUT/DELETE; path is the
// resource path under the API base beginning with "/" (e.g. "/boards/abc").
// params carries the operation's parameters (both filters and write fields) —
// the API key and token are appended here, so callers never include them. There
// is no request body: Trello reads write parameters from the query string.
func Do(a Auth, method, path string, params url.Values) (*APIResponse, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("key", a.Key)
	params.Set("token", a.Token)

	fullURL := apiBase + path + "?" + params.Encode()

	req, err := http.NewRequest(method, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", redactAuth(a, err.Error()))
	}
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Trello API request failed: %s", redactAuth(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	return &APIResponse{StatusCode: resp.StatusCode, Body: body, Headers: resp.Header}, nil
}

// redactAuth removes the API key and token from an error message. They travel in
// the query string, so a wrapped transport error could echo the full URL — they
// are scrubbed defensively.
func redactAuth(a Auth, msg string) string {
	if a.Key != "" {
		msg = strings.ReplaceAll(msg, a.Key, "REDACTED")
	}
	if a.Token != "" {
		msg = strings.ReplaceAll(msg, a.Token, "REDACTED")
	}
	return msg
}

// CheckResponse verifies a 2xx status. Trello returns validation failures as a
// short plain-text body (e.g. "invalid value for name") rather than a JSON
// envelope, so the body is surfaced directly (trimmed and key/token-free by
// construction — they are never echoed in the message body).
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body := strings.TrimSpace(string(resp.Body))
	if len(body) > 500 {
		body = body[:500]
	}
	if body == "" {
		body = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("Trello API error (%d): %s", resp.StatusCode, body)
}

// decodeObject unmarshals a successful single-resource body into a generic map.
// An empty body yields an empty map.
func decodeObject(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Trello response: %w", err)
	}
	return out, nil
}

// decodeArray unmarshals a successful list body into a generic slice.
func decodeArray(resp *APIResponse) ([]interface{}, error) {
	if len(resp.Body) == 0 {
		return []interface{}{}, nil
	}
	var out []interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Trello list response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Generic resource helpers
// ---------------------------------------------------------------------------

// GetObject GETs a single resource and decodes the bare returned object.
func GetObject(a Auth, path string, params url.Values) (map[string]interface{}, error) {
	resp, err := Do(a, http.MethodGet, path, params)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// GetArray GETs a list resource and decodes the bare returned array.
func GetArray(a Auth, path string, params url.Values) ([]interface{}, error) {
	resp, err := Do(a, http.MethodGet, path, params)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeArray(resp)
}

// WriteObject performs a POST or PUT with parameters in the query string and
// decodes the returned object (Trello echoes the created/updated resource).
func WriteObject(a Auth, method, path string, params url.Values) (map[string]interface{}, error) {
	resp, err := Do(a, method, path, params)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// WriteOK performs a POST/PUT and verifies success WITHOUT decoding the body.
// Used by writes whose response is not a single object (e.g. adding a label to a
// card returns the card's new array of idLabels) and whose returned body the
// caller does not need. Avoids a spurious "cannot unmarshal array into map"
// error on a call that actually succeeded.
func WriteOK(a Auth, method, path string, params url.Values) error {
	resp, err := Do(a, method, path, params)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// DeleteOK DELETEs a resource and verifies success WITHOUT decoding the body.
// Some Trello delete endpoints (e.g. removing a checklist from a card) return a
// JSON array rather than an object; the caller only needs the 2xx, so the body
// is not parsed.
func DeleteOK(a Auth, path string, params url.Values) error {
	resp, err := Do(a, http.MethodDelete, path, params)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
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

// OptionalBoolSet reports an optional boolean and whether it was set at all
// (checkbox touched), preserving the tri-state nil as "omit".
func OptionalBoolSet(name string, inputs []*core.Connection) (val, set bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false, false
	}
	return *conn.Boolean(), true
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

// SetIfPresent adds an optional string parameter only when provided. field is
// the Trello query-param name; inputName is the action's input name.
func SetIfPresent(params url.Values, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		params.Set(field, v)
	}
}

// SetBoolIfSet adds an optional boolean parameter (as "true"/"false") only when
// its input connection is present.
func SetBoolIfSet(params url.Values, inputs []*core.Connection, field, inputName string) {
	if v, ok := OptionalBoolSet(inputName, inputs); ok {
		params.Set(field, strconv.FormatBool(v))
	}
}

// SetIntIfPresent parses an optional string/integer input as an integer and adds
// it as a parameter when present. A non-numeric value is surfaced as an error.
func SetIntIfPresent(params url.Values, inputs []*core.Connection, field, inputName string) error {
	if n, ok := OptionalInt(inputName, inputs); ok {
		params.Set(field, strconv.Itoa(n))
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
	params.Set(field, strconv.Itoa(n))
	return nil
}

// MergeAdditionalFields overlays a raw JSON object input ("additional_fields")
// onto the params — the escape hatch for any Trello field not exposed as a
// first-class input (e.g. board prefs_*). Called LAST in param assembly, so a
// key here OVERRIDES the same key set by a first-class input (the "power-user
// last word" precedence shared with the WordPress / WooCommerce / Jira nodes).
// Scalar values are stringified; nested objects/arrays are JSON-encoded (Trello
// has no nested query params, so this is the pragmatic representation).
func MergeAdditionalFields(params url.Values, inputs []*core.Connection) error {
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
		params.Set(k, stringifyParam(val))
	}
	return nil
}

// stringifyParam renders a JSON value as a query-parameter string. Strings pass
// through; numbers/bools are formatted plainly; objects/arrays are JSON-encoded.
func stringifyParam(v interface{}) string {
	switch t := v.(type) {
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		// Render integers without a trailing ".0".
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case nil:
		return ""
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		return string(b)
	}
}

// SetFieldsParam adds a comma-separated "fields" filter when the input is
// present, mapping to Trello's `fields` query parameter (which narrows the
// returned attributes). Multi-value editor inputs are single-select, so this is
// entered as comma-separated text.
func SetFieldsParam(params url.Values, inputs []*core.Connection, inputName string) {
	SetIfPresent(params, inputs, "fields", inputName)
}

// ClampLimit bounds a requested page size to Trello's 1-1000 range, falling back
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
// Result shaping
// ---------------------------------------------------------------------------

// ErrorResult is the standard soft-failure output map (returned with a nil error
// so the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ResourceResult shapes a single-resource response (create/get/update) into the
// standard action output. The id output reads the resource's "id".
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          stringifyID(obj["id"]),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// SuccessResult shapes an operation whose id the caller already knows (delete, or
// a call whose body is a bare confirmation). result carries any small map.
func SuccessResult(id string, result map[string]interface{}, summary string) map[string]interface{} {
	if result == nil {
		result = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          id,
		"result":      result,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output.
func ListResult(items []interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"total":       len(items),
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// stringifyID renders a Trello id (a hex string, but numeric ids are handled for
// safety) as a clean string.
func stringifyID(id interface{}) string {
	switch v := id.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}
