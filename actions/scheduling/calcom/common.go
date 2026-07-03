// Package calcom holds the shared HTTP client, auth helper, and generic
// request/pagination/result plumbing used by every scheduling/calcom/* action.
//
// Cal.com's API v2 (https://api.cal.com/v2) is uniform in shape: every response
// is wrapped in a {"status": "success", "data": ...} envelope (errors use
// {"status": "error", "error": {"code", "message", ...}}); single resources put
// the object in data, collections put a bare array in data with an optional
// top-level {"pagination": {...}} block. That regularity lets the request,
// pagination and result shaping live here once so each action package stays
// thin: read its inputs, call one helper, shape the result.
//
// Two things differ from the sibling Calendly client and are the reason this is
// its own package rather than shared code:
//
//   - Versioning. Cal.com pins the request/response schema per endpoint group
//     via a `cal-api-version` date-string header. Omitting it silently returns
//     a legacy schema, so every call threads the right Version* constant.
//   - Pagination. Collections page by `take`/`skip` (offset), and whether more
//     pages exist is read from `pagination.hasNextPage`. Endpoints that return
//     a full array (schedules, teams, event-types) simply omit the pagination
//     block, which ListResources treats as a single page.
//
// Auth is a Bearer API key (cal_live_...) in the Authorization header. The token
// input is a ConnectionTypeSecret, which accepts both a managed Cal.com
// credential (${credentials.X} — platform-refreshed OAuth) and a pasted API key
// (${secrets.X}); both resolve to a plain token at run time, so one input covers
// both auth modes.
package calcom

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
	// maxResponseBody caps the response body to prevent memory exhaustion.
	// Booking collections can be large, so 8 MB (the airtable/calendly value)
	// rather than 1 MB.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single Cal.com call.
	requestTimeout = 30 * time.Second

	// MaxAllPages bounds a "return all" pagination loop so an account with a
	// huge history can never spin unbounded requests. On hitting the cap the
	// action returns the outstanding skip offset so the caller resumes.
	MaxAllPages = 100

	// DefaultPageLimit / MaxPageLimit bound Cal.com's per-page `take` count.
	// Cal.com accepts take up to 250 (memberships/webhooks); 100 is a friendly
	// default page size.
	DefaultPageLimit = 100
	MaxPageLimit     = 250
)

// cal-api-version pins. Each endpoint group is fixed to the schema version this
// client was written against; see the per-resource docs. Groups with no
// documented version (teams, memberships, webhooks, me) send no header and use
// VersionNone.
const (
	VersionNone       = ""
	VersionBookings   = "2026-02-25"
	VersionEventTypes = "2024-06-14"
	VersionSchedules  = "2024-06-11"
	VersionSlots      = "2024-09-04"
)

// httpClient is shared across every Cal.com action so TCP connections to
// api.cal.com are pooled and reused rather than re-dialled per call.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// apiBaseURL is a var rather than a const so tests can point every request at
// an httptest server (the same seam idiom as calendly's apiBaseURL). It also
// leaves room for a future self-hosted host override.
var apiBaseURL = "https://api.cal.com/v2"

// SetBaseURLForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores the real host. It lets action
// packages in sibling directories exercise Execute end-to-end without hitting
// Cal.com. Test-only.
func SetBaseURLForTest(base string) func() {
	prev := apiBaseURL
	apiBaseURL = base
	return func() { apiBaseURL = prev }
}

// AuthInput is the shared credential input every action puts first. Action
// packages re-declare their own literal Inputs arrays (the manifest generator
// AST-parses those); this documents the canonical shape.
var AuthInput = core.Connection{
	Name:        "api_key",
	Type:        core.ConnectionTypeSecret,
	Label:       "Cal.com Connection",
	Placeholder: "Cal.com API key (cal_live_...) or connected credential",
	Required:    true,
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
}

// GetAuth resolves the Bearer token from the action's auth input.
func GetAuth(inputs []*core.Connection) (string, error) {
	token, err := RequiredString("api_key", inputs)
	if err != nil {
		return "", fmt.Errorf("authentication required: connect a Cal.com credential or supply an API key")
	}
	return token, nil
}

// ExecuteAPI performs a REST call against the Cal.com API v2.
//
//	method:     GET, POST, PATCH, DELETE
//	path:       resource path under the v2 base, including any query string
//	            (e.g. "/event-types?username=dave")
//	apiVersion: the cal-api-version pin for this endpoint group ("" to omit)
//	body:       optional payload — marshalled to JSON for POST/PATCH, ignored
//	            otherwise
func ExecuteAPI(token, method, path, apiVersion string, body interface{}) (*APIResponse, error) {
	fullURL := apiBaseURL + path

	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPatch) {
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

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if apiVersion != "" {
		req.Header.Set("cal-api-version", apiVersion)
	}
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cal.com API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody}, nil
}

// CheckResponse verifies a 2xx status, decoding Cal.com's error envelope
// ({"status":"error","error":{"code":"...","message":"...","details":{...}}}).
// 429 is surfaced explicitly so the caller understands it hit Cal.com's rate
// limit.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("Cal.com API rate limit exceeded (429)")
	}

	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &env); err == nil && (env.Error.Message != "" || env.Error.Code != "") {
		msg := env.Error.Message
		if msg == "" {
			msg = env.Error.Code
		} else if env.Error.Code != "" {
			msg = env.Error.Code + ": " + msg
		}
		return fmt.Errorf("Cal.com API error (%d): %s", resp.StatusCode, msg)
	}
	return fmt.Errorf("Cal.com API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// envelope is the standard v2 wrapper. data is decoded lazily by the callers
// below since it is an object for single resources and an array for
// collections.
type envelope struct {
	Status     string          `json:"status"`
	Data       json.RawMessage `json:"data"`
	Pagination json.RawMessage `json:"pagination"`
}

// decodeObject unwraps a single-resource response into its data object. An
// empty body (e.g. a delete's 204/empty 200) yields an empty map.
func decodeObject(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var env envelope
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return nil, fmt.Errorf("failed to parse Cal.com response: %w", err)
	}
	out := map[string]interface{}{}
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &out); err != nil {
			// data was not an object (some ops return null or a scalar); fall
			// back to an empty map rather than erroring the whole action.
			return map[string]interface{}{}, nil
		}
	}
	return out, nil
}

// decodePage unwraps a collection response into its data array plus the raw
// pagination block (nil when the endpoint returns a full array with no
// pagination). A non-array data field yields an empty slice.
func decodePage(resp *APIResponse) (items []interface{}, hasNext bool, err error) {
	if len(resp.Body) == 0 {
		return []interface{}{}, false, nil
	}
	var env envelope
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return nil, false, fmt.Errorf("failed to parse Cal.com response: %w", err)
	}
	items = []interface{}{}
	if len(env.Data) > 0 {
		_ = json.Unmarshal(env.Data, &items) // non-array data → empty slice
	}
	if len(env.Pagination) > 0 {
		var p struct {
			HasNextPage    bool `json:"hasNextPage"`
			RemainingItems int  `json:"remainingItems"`
		}
		if json.Unmarshal(env.Pagination, &p) == nil {
			hasNext = p.HasNextPage || p.RemainingItems > 0
		}
	}
	return items, hasNext, nil
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

// OptionalInt extracts an integer input. The bool is false when absent, so
// callers distinguish "unset" from "set to 0".
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Number() == nil {
		return 0, false
	}
	return int(*conn.Number()), true
}

// RequiredInt extracts a required integer input, erroring if absent. Mirrors
// RequiredString so required numeric ids (event type, schedule, team, ...) are
// validated with the same pattern rather than an ad-hoc OptionalInt + !ok.
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

// OptionalStringSlice reads a multi-select (or comma-separated string) input
// into a slice of trimmed, non-empty values. Handles the three shapes a
// selection can arrive as: a []interface{} (parsed multi-select), a JSON array
// string, or a plain comma-separated string.
func OptionalStringSlice(name string, inputs []*core.Connection) []string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return nil
	}
	split := func(s string) []string {
		var out []string
		for _, p := range strings.Split(s, ",") {
			if t := strings.TrimSpace(p); t != "" {
				out = append(out, t)
			}
		}
		return out
	}
	switch v := conn.Value.(type) {
	case []interface{}:
		var out []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				if t := strings.TrimSpace(s); t != "" {
					out = append(out, t)
				}
			}
		}
		return out
	case string:
		s := strings.TrimSpace(v)
		if strings.HasPrefix(s, "[") {
			var arr []string
			if json.Unmarshal([]byte(s), &arr) == nil {
				var out []string
				for _, a := range arr {
					if t := strings.TrimSpace(a); t != "" {
						out = append(out, t)
					}
				}
				return out
			}
		}
		return split(s)
	default:
		if str := conn.String(); str != nil {
			return split(*str)
		}
	}
	return nil
}

// ParseJSONObject reads an "advanced" JSON object input (ConnectionTypeObject).
// The value arrives either already-parsed (map) or as a JSON string. Returns
// nil (not an error) when the input is absent/empty.
func ParseJSONObject(name string, inputs []*core.Connection) (map[string]interface{}, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return nil, nil
	}
	switch v := conn.Value.(type) {
	case map[string]interface{}:
		if len(v) == 0 {
			return nil, nil
		}
		return v, nil
	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return nil, nil
		}
		var out map[string]interface{}
		if err := json.Unmarshal([]byte(s), &out); err != nil {
			return nil, fmt.Errorf("%s must be a valid JSON object: %w", name, err)
		}
		return out, nil
	default:
		if str := conn.String(); str != nil && strings.TrimSpace(*str) != "" {
			var out map[string]interface{}
			if err := json.Unmarshal([]byte(*str), &out); err != nil {
				return nil, fmt.Errorf("%s must be a valid JSON object: %w", name, err)
			}
			return out, nil
		}
	}
	return nil, nil
}

// ParseJSONArray reads an "advanced" JSON array input (e.g. a schedule's
// availability blocks). Returns nil (not an error) when absent/empty.
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

// AddFilter sets a query param from an optional string input when non-empty.
func AddFilter(q url.Values, inputs []*core.Connection, param, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		q.Set(param, v)
	}
}

// SetIfString adds key=value to body when the named string input is non-empty.
func SetIfString(body map[string]interface{}, inputs []*core.Connection, key, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[key] = v
	}
}

// SetIfInt adds key=value to body when the named integer input is set.
func SetIfInt(body map[string]interface{}, inputs []*core.Connection, key, inputName string) {
	if v, ok := OptionalInt(inputName, inputs); ok {
		body[key] = v
	}
}

// SetIfBoolPresent adds key=value to body only when the named boolean input is
// explicitly present (so an unchecked box doesn't force false onto an update).
func SetIfBoolPresent(body map[string]interface{}, inputs []*core.Connection, key, inputName string) {
	if conn := core.FindConnection(inputName, inputs); conn != nil && conn.Boolean() != nil {
		body[key] = *conn.Boolean()
	}
}

// ClampLimit bounds a requested per-page count to Cal.com's 1-250 range,
// falling back to DefaultPageLimit when unset.
func ClampLimit(limit int, set bool) int {
	if !set || limit <= 0 {
		return DefaultPageLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit
	}
	return limit
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

// GetResource GETs a single resource by path, applying optional query params.
func GetResource(token, path, apiVersion string, q url.Values) (map[string]interface{}, error) {
	if len(q) > 0 {
		path += "?" + q.Encode()
	}
	resp, err := ExecuteAPI(token, http.MethodGet, path, apiVersion, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// PostResource POSTs a payload to a path and decodes the response object.
func PostResource(token, path, apiVersion string, payload interface{}) (map[string]interface{}, error) {
	resp, err := ExecuteAPI(token, http.MethodPost, path, apiVersion, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// PatchResource PATCHes a payload to a path and decodes the response object.
func PatchResource(token, path, apiVersion string, payload interface{}) (map[string]interface{}, error) {
	resp, err := ExecuteAPI(token, http.MethodPatch, path, apiVersion, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// DeleteResource DELETEs a single resource by path.
func DeleteResource(token, path, apiVersion string) error {
	resp, err := ExecuteAPI(token, http.MethodDelete, path, apiVersion, nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ListResources fetches a collection with take/skip pagination. When returnAll
// is false a single page (starting at the given skip) is fetched and the next
// skip offset is returned when more pages exist (0 when drained). When true it
// walks pages until pagination.hasNextPage is false or the MaxAllPages cap.
// Endpoints that return a full array with no pagination block yield a single
// page. Returns the accumulated items, the outstanding next skip offset, and
// the number of pages fetched.
func ListResources(token, resourcePath, apiVersion string, q url.Values, take, skip int, returnAll bool) (items []interface{}, nextSkip, pages int, err error) {
	// Non-nil so a zero-match list serialises as [] not null — get-many is
	// consumed by Loop nodes that iterate the array.
	items = []interface{}{}
	if q == nil {
		q = url.Values{}
	}
	if take <= 0 {
		take = DefaultPageLimit
	}
	if skip < 0 {
		skip = 0
	}

	for {
		pq := cloneValues(q)
		pq.Set("take", strconv.Itoa(take))
		pq.Set("skip", strconv.Itoa(skip))
		path := resourcePath + "?" + pq.Encode()

		resp, e := ExecuteAPI(token, http.MethodGet, path, apiVersion, nil)
		if e != nil {
			return nil, 0, pages, e
		}
		if e := CheckResponse(resp); e != nil {
			return nil, 0, pages, e
		}
		pageItems, hasNext, e := decodePage(resp)
		if e != nil {
			return nil, 0, pages, e
		}
		items = append(items, pageItems...)
		pages++
		skip += take

		if !hasNext {
			return items, 0, pages, nil
		}
		if !returnAll {
			return items, skip, pages, nil
		}
		if pages >= MaxAllPages {
			return items, skip, pages, nil
		}
	}
}

func cloneValues(q url.Values) url.Values {
	out := url.Values{}
	for k, vs := range q {
		for _, v := range vs {
			out.Add(k, v)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// idOf returns the resource's identifier as a string, preferring the string
// "uid" (bookings) then the numeric "id" (event-types, schedules, teams, ...).
func idOf(obj map[string]interface{}) string {
	if uid, ok := obj["uid"].(string); ok && uid != "" {
		return uid
	}
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

// ResourceResult shapes a single-resource response (get/create/update/action)
// into the standard action output. id is the resource uid/id.
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	return map[string]interface{}{
		"id":          idOf(obj),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection into the standard list output: results array,
// count, the outstanding next skip offset, plus the mandatory
// summary/success/error triple.
func ListResult(items []interface{}, nextSkip int, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"next_skip":   nextSkip,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}
