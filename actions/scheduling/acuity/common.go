// Package acuity holds the shared HTTP client, auth helper, and generic
// request/result plumbing used by every scheduling/acuity/* action.
//
// Acuity Scheduling's API v1 (https://acuityscheduling.com/api/v1) is simple and
// uniform: single resources come back as a bare JSON object, collections as a
// bare JSON array (no envelope, no pagination wrapper — list size is bounded by
// a `max` query param and date filters). Errors are a small
// {"status_code","message","error"} object. That regularity lets the request
// and result shaping live here once so each action package stays thin.
//
// Auth is HTTP Basic: the Acuity **User ID** is the Basic username and the
// **API Key** is the Basic password (the same pairing n8n uses). Both are
// separate inputs — user_id is a plain string, api_key a ConnectionTypeSecret —
// because Basic needs both halves. (Acuity also offers OAuth2 scope api-v1;
// that would be a separate managed-credential path, not wired here.)
//
// NOTE: Acuity's API is gated to the Powerhouse plan — every endpoint returns
// 403 "API access is only available on Powerhouse plans" on lower tiers.
// CheckResponse surfaces that verbatim so the cause is obvious at runtime.
package acuity

import (
	"bytes"
	"encoding/base64"
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
	// maxResponseBody caps the response body to prevent memory exhaustion.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single Acuity call.
	requestTimeout = 30 * time.Second

	// DefaultListMax / MaxListMax bound the `max` result count on list
	// endpoints (Acuity defaults to 100).
	DefaultListMax = 100
	MaxListMax     = 1000
)

// httpClient is shared across every Acuity action so TCP connections to
// acuityscheduling.com are pooled and reused rather than re-dialled per call.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// apiBaseURL is a var rather than a const so tests can point every request at
// an httptest server (the same seam idiom as the calcom/calendly clients).
var apiBaseURL = "https://acuityscheduling.com/api/v1"

// SetBaseURLForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores the real host. Test-only.
func SetBaseURLForTest(base string) func() {
	prev := apiBaseURL
	apiBaseURL = base
	return func() { apiBaseURL = prev }
}

// UserIDInput / APIKeyInput document the canonical auth inputs every action
// puts first (action packages re-declare them literally for the AST manifest
// generator).
var UserIDInput = core.Connection{
	Name:     "user_id",
	Type:     core.ConnectionTypeString,
	Label:    "Acuity User ID",
	Required: true,
}

var APIKeyInput = core.Connection{
	Name:        "api_key",
	Type:        core.ConnectionTypeSecret,
	Label:       "Acuity API Key",
	Placeholder: "Acuity API key (Basic-auth password)",
	Required:    true,
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
}

// GetAuth resolves the Acuity User ID + API Key from the action's inputs.
func GetAuth(inputs []*core.Connection) (userID, apiKey string, err error) {
	userID = OptionalString("user_id", inputs)
	apiKey = OptionalString("api_key", inputs)
	if userID == "" || apiKey == "" {
		return "", "", fmt.Errorf("authentication required: supply the Acuity User ID and API Key")
	}
	return userID, apiKey, nil
}

// ExecuteAPI performs a REST call against the Acuity API v1 using HTTP Basic
// auth (userID:apiKey).
//
//	method: GET, POST, PUT, DELETE
//	path:   resource path including any query string (e.g. "/appointments?max=1")
//	body:   optional payload — marshalled to JSON for POST/PUT, ignored otherwise
func ExecuteAPI(userID, apiKey, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := apiBaseURL + path

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

	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(userID+":"+apiKey)))
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Acuity API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody}, nil
}

// CheckResponse verifies a 2xx status, decoding Acuity's error envelope
// ({"status_code","message","error"}). 429 is surfaced explicitly.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("Acuity API rate limit exceeded (429)")
	}

	var env struct {
		StatusCode int    `json:"status_code"`
		Message    string `json:"message"`
		Error      string `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &env); err == nil && (env.Message != "" || env.Error != "") {
		msg := env.Message
		if msg == "" {
			msg = env.Error
		}
		return fmt.Errorf("Acuity API error (%d): %s", resp.StatusCode, msg)
	}
	return fmt.Errorf("Acuity API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// decodeObject unmarshals a single-resource (or single-object) response into a
// generic map. An empty body yields an empty map.
func decodeObject(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Acuity response: %w", err)
	}
	return out, nil
}

// decodeArray unmarshals a collection response (a bare JSON array) into a slice.
// A non-array body yields an empty slice.
func decodeArray(resp *APIResponse) []interface{} {
	items := []interface{}{}
	if len(resp.Body) == 0 {
		return items
	}
	_ = json.Unmarshal(resp.Body, &items)
	return items
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

// RequiredInt extracts a required integer input, erroring if absent.
func RequiredInt(name string, inputs []*core.Connection) (int, error) {
	v, ok := OptionalInt(name, inputs)
	if !ok {
		return 0, fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// OptionalBool extracts a boolean input, defaulting to false when unset.
func OptionalBool(name string, inputs []*core.Connection) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false
	}
	return *conn.Boolean()
}

// AddFilter sets a query param from an optional string input when non-empty.
func AddFilter(q url.Values, inputs []*core.Connection, param, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		q.Set(param, v)
	}
}

// AddIntFilter sets a query param from an optional integer input when set.
func AddIntFilter(q url.Values, inputs []*core.Connection, param, inputName string) {
	if v, ok := OptionalInt(inputName, inputs); ok {
		q.Set(param, strconv.Itoa(v))
	}
}

// SetIfString adds key=value to body when the named string input is non-empty.
func SetIfString(body map[string]interface{}, inputs []*core.Connection, key, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[key] = v
	}
}

// SetIfBoolPresent adds key=value to body only when the named boolean input is
// explicitly present.
func SetIfBoolPresent(body map[string]interface{}, inputs []*core.Connection, key, inputName string) {
	if conn := core.FindConnection(inputName, inputs); conn != nil && conn.Boolean() != nil {
		body[key] = *conn.Boolean()
	}
}

// ParseJSONArray reads an "advanced" JSON array input (e.g. intake-form field
// answers). Returns nil (not an error) when absent/empty.
func ParseJSONArray(name string, inputs []*core.Connection) ([]interface{}, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return nil, nil
	}
	if arr, ok := conn.Value.([]interface{}); ok {
		return arr, nil
	}
	str := conn.String()
	if str == nil || strings.TrimSpace(*str) == "" {
		return nil, nil
	}
	var out []interface{}
	if err := json.Unmarshal([]byte(*str), &out); err != nil {
		return nil, fmt.Errorf("%s must be a valid JSON array: %w", name, err)
	}
	return out, nil
}

// ClampMax bounds a requested `max` list count to Acuity's 1-1000 range,
// falling back to DefaultListMax when unset.
func ClampMax(max int, set bool) int {
	if !set || max <= 0 {
		return DefaultListMax
	}
	if max > MaxListMax {
		return MaxListMax
	}
	return max
}

// ErrorResult is the standard soft-failure output map (returned with a nil
// error so the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ---------------------------------------------------------------------------
// Generic resource access
// ---------------------------------------------------------------------------

// GetObject GETs a single resource (or single-object endpoint like /me) by path.
func GetObject(userID, apiKey, path string, q url.Values) (map[string]interface{}, error) {
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	resp, err := ExecuteAPI(userID, apiKey, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// GetList GETs a collection (bare array) by path.
func GetList(userID, apiKey, path string, q url.Values) ([]interface{}, error) {
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	resp, err := ExecuteAPI(userID, apiKey, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeArray(resp), nil
}

// PostObject POSTs a payload to a path and decodes the response object.
func PostObject(userID, apiKey, path string, q url.Values, payload interface{}) (map[string]interface{}, error) {
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	resp, err := ExecuteAPI(userID, apiKey, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// PostList POSTs a payload and decodes an array response (e.g.
// /availability/check-times returns an array of slot validity results).
func PostList(userID, apiKey, path string, payload interface{}) ([]interface{}, error) {
	resp, err := ExecuteAPI(userID, apiKey, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeArray(resp), nil
}

// PutObject PUTs a payload to a path and decodes the response object.
func PutObject(userID, apiKey, path string, q url.Values, payload interface{}) (map[string]interface{}, error) {
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	resp, err := ExecuteAPI(userID, apiKey, http.MethodPut, path, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// DeleteResource DELETEs a resource by path, applying optional query params.
func DeleteResource(userID, apiKey, path string, q url.Values) error {
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	resp, err := ExecuteAPI(userID, apiKey, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// idOf returns the resource's numeric "id" as a string.
func idOf(obj map[string]interface{}) string {
	switch v := obj["id"].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case json.Number:
		return v.String()
	}
	return ""
}

// ResourceResult shapes a single-resource response into the standard action
// output. id is the resource id (empty for id-less endpoints like /me).
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	return map[string]interface{}{
		"id":          idOf(obj),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection into the standard list output.
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
