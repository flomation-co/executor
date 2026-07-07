// Package asana_common holds the shared HTTP client, auth helpers, and generic
// resource helpers used by every asana/* action.
//
// Asana's REST API (v1.0) is small and very regular. Everything hangs off a
// single base — https://app.asana.com/api/1.0 — and every resource (tasks,
// subtasks, projects, sections, tags, users, workspaces) follows conventional
// create/read/update/delete/list shapes. That regularity lets the transport,
// auth, error handling and result-shaping live here once, so each action package
// stays thin — the same design as the sibling jira / trello packages.
//
// Four things shape this file:
//
//   - Auth is a Personal Access Token sent as an HTTP Bearer header. It is a
//     ConnectionTypeSecret (rendered as an env-secret picker), and is scrubbed
//     from any error string. The base host is FIXED (app.asana.com), never
//     caller-supplied, so there is no SSRF surface and no insecure path to gate.
//
//   - Asana wraps EVERYTHING in a "data" envelope. Request bodies for POST/PUT
//     are sent as {"data": {...fields}}; responses come back as {"data": {...}}
//     for a single resource or {"data": [...]} for a list. The helpers below own
//     that wrapping/unwrapping so actions deal in plain maps.
//
//   - IDs are opaque "gid" strings. The id output reads obj["gid"].
//
//   - Lists paginate with an opaque cursor: a page response carries
//     "next_page": {"offset": "...", "uri": "..."} (or null on the last page).
//     ListAll follows next_page.offset to the page cap; a single page uses limit.
package asana_common

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
	// APIBase is the fixed Asana REST API root. It is a constant (never
	// caller-supplied), so every request targets app.asana.com and there is no
	// SSRF surface.
	APIBase = "https://app.asana.com/api/1.0"

	// maxResponseBody caps the response body to prevent memory exhaustion.
	maxResponseBody = 8 << 20 // 8 MB

	requestTimeout = 30 * time.Second

	// DefaultPageLimit / MaxPageLimit bound a single list page. Asana caps most
	// list endpoints at 100 per page.
	DefaultPageLimit = 100
	MaxPageLimit     = 100

	// MaxAllPages bounds a "return all" pagination loop so a huge project can
	// never spin unbounded requests.
	MaxAllPages = 100
)

// httpClient is shared across every Asana action so TLS connections to
// app.asana.com are pooled and reused rather than re-dialled per call.
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
// The secret is named access_token (not "token"/"password") so a resource field
// never collides with it: core.FindConnection returns the FIRST input whose name
// matches, so a resource field sharing a credential's name would silently
// resolve to the credential. No Asana resource field uses that name.
var AuthInputs = []core.Connection{
	{
		Name:        "access_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Access Token",
		Placeholder: "Your Asana Personal Access Token (app.asana.com ▸ Settings ▸ Apps ▸ Developer console)",
		Required:    true,
	},
}

// Auth is the resolved credential: the Personal Access Token.
type Auth struct {
	Token string
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// GetAuth resolves the access token from the action's auth inputs.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	token, err := RequiredString("access_token", inputs)
	if err != nil {
		return Auth{}, err
	}
	return Auth{Token: token}, nil
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

// Do performs a REST call to Asana. method is GET/POST/PUT/DELETE; path is the
// resource path under the API base beginning with "/". bodyData, when non-nil,
// is the write payload — it is wrapped as {"data": bodyData} (Asana's envelope)
// and sent for POST/PUT. query carries GET filters / opt_fields / pagination.
func Do(a Auth, method, path string, bodyData map[string]interface{}, query url.Values) (*APIResponse, error) {
	fullURL := apiBase + path
	if enc := query.Encode(); enc != "" {
		fullURL += "?" + enc
	}

	var bodyReader io.Reader
	if bodyData != nil && (method == http.MethodPost || method == http.MethodPut) {
		b, err := json.Marshal(map[string]interface{}{"data": bodyData})
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", redactAuth(a, err.Error()))
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Asana API request failed: %s", redactAuth(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	return &APIResponse{StatusCode: resp.StatusCode, Body: body, Headers: resp.Header}, nil
}

// redactAuth removes the access token from an error message. It travels in the
// Authorization header (not the URL), so a leak is unlikely — but a wrapped
// error could echo it, so it is scrubbed defensively.
func redactAuth(a Auth, msg string) string {
	if a.Token != "" {
		msg = strings.ReplaceAll(msg, a.Token, "REDACTED")
	}
	return msg
}

// CheckResponse verifies a 2xx status, decoding Asana's error envelope
// ({"errors":[{"message":"...","help":"..."}]}). All messages are surfaced.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var env struct {
		Errors []struct {
			Message string `json:"message"`
			Help    string `json:"help"`
		} `json:"errors"`
	}
	parts := []string{}
	if err := json.Unmarshal(resp.Body, &env); err == nil {
		for _, e := range env.Errors {
			if e.Message != "" {
				parts = append(parts, e.Message)
			}
		}
	}
	if len(parts) > 0 {
		return fmt.Errorf("Asana API error (%d): %s", resp.StatusCode, strings.Join(parts, "; "))
	}
	body := strings.TrimSpace(string(resp.Body))
	if len(body) > 500 {
		body = body[:500]
	}
	if body == "" {
		body = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("Asana API error (%d): %s", resp.StatusCode, body)
}

// dataEnvelope is the shape every Asana response shares: the payload under
// "data" plus an optional pagination cursor.
type dataEnvelope struct {
	Data     json.RawMessage `json:"data"`
	NextPage *struct {
		Offset string `json:"offset"`
		URI    string `json:"uri"`
	} `json:"next_page"`
}

// decodeObject unwraps {"data": {...}} into a generic map. An empty body yields
// an empty map (e.g. a delete returning {"data":{}}).
func decodeObject(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var env dataEnvelope
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return nil, fmt.Errorf("failed to parse Asana response: %w", err)
	}
	if len(env.Data) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(env.Data, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Asana response data: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Generic resource helpers
// ---------------------------------------------------------------------------

// GetObject GETs a single resource and unwraps the data object.
func GetObject(a Auth, path string, query url.Values) (map[string]interface{}, error) {
	resp, err := Do(a, http.MethodGet, path, nil, query)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// WriteObject performs a POST or PUT with the given data fields and unwraps the
// returned data object (Asana echoes the created/updated resource).
func WriteObject(a Auth, method, path string, fields map[string]interface{}, query url.Values) (map[string]interface{}, error) {
	resp, err := Do(a, method, path, fields, query)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// DeleteResource DELETEs a resource. Asana returns {"data":{}} on success.
func DeleteResource(a Auth, path string) error {
	resp, err := Do(a, http.MethodDelete, path, nil, url.Values{})
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ListPage fetches a single page of a collection and returns the items plus the
// next-page offset cursor ("" when there are no more pages).
func ListPage(a Auth, path string, query url.Values) ([]interface{}, string, error) {
	resp, err := Do(a, http.MethodGet, path, nil, query)
	if err != nil {
		return nil, "", err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, "", err
	}
	var env dataEnvelope
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return nil, "", fmt.Errorf("failed to parse Asana list response: %w", err)
	}
	items := []interface{}{}
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &items); err != nil {
			return nil, "", fmt.Errorf("failed to parse Asana list data: %w", err)
		}
	}
	offset := ""
	if env.NextPage != nil {
		offset = env.NextPage.Offset
	}
	return items, offset, nil
}

// ListPageOnly fetches a single page and returns just the items, discarding the
// pagination cursor. Used by endpoints that don't support offset pagination
// (e.g. the task-search endpoint, which returns up to a capped result set).
func ListPageOnly(a Auth, path string, query url.Values) ([]interface{}, error) {
	items, _, err := ListPage(a, path, query)
	return items, err
}

// ListAll fetches a collection. When returnAll is false a single page (size
// limit) is returned; when true it follows the offset cursor to the MaxAllPages
// cap. Extra filters/opt_fields come in via query.
func ListAll(a Auth, path string, query url.Values, limit int, returnAll bool) ([]interface{}, error) {
	if query == nil {
		query = url.Values{}
	}
	all := []interface{}{}
	pageSize := ClampLimit(limit, limit > 0)
	query.Set("limit", strconv.Itoa(pageSize))
	pages := 0
	for {
		items, offset, err := ListPage(a, path, query)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		pages++
		if !returnAll || offset == "" || len(items) == 0 || pages >= MaxAllPages {
			break
		}
		query.Set("offset", offset)
	}
	return all, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}

func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Number() == nil {
		return 0, false
	}
	return int(*conn.Number()), true
}

func OptionalBoolSet(name string, inputs []*core.Connection) (val, set bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false, false
	}
	return *conn.Boolean(), true
}

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

// SetIfPresent adds an optional string field to the data body only when provided.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// SetBoolIfSet adds an optional boolean field when its input connection is
// present (checkbox touched), preserving the tri-state nil as "omit".
func SetBoolIfSet(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v, ok := OptionalBoolSet(inputName, inputs); ok {
		body[field] = v
	}
}

// SetStringListIfPresent splits a comma-separated input into a []string and adds
// it to the body when present. Asana array fields (projects, followers, tags)
// accept a JSON array; editor multi-value inputs are single-select, so these are
// entered as comma-separated text.
func SetStringListIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if list := StringList(inputName, inputs); len(list) > 0 {
		body[field] = list
	}
}

// StringList splits a comma-separated input into a trimmed []string.
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
// onto the data body — the escape hatch for any Asana field not exposed as a
// first-class input. Called LAST so a key here OVERRIDES a first-class input
// (the "power-user last word" precedence shared with the sibling nodes).
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

// SetOptFields maps an optional comma-separated "opt_fields" input to Asana's
// opt_fields query parameter, which selects which attributes the response
// includes (Asana returns a compact object by default).
func SetOptFields(query url.Values, inputs []*core.Connection, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		query.Set("opt_fields", v)
	}
}

// ClampLimit bounds a requested page size to Asana's 1-100 range, falling back
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

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ResourceResult shapes a single-resource response (create/get/update) into the
// standard action output. The id output reads the resource's "gid".
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          stringifyID(obj["gid"]),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// SuccessResult shapes an operation whose id the caller already knows (delete /
// add / remove side calls that return an empty or confirmation body).
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
