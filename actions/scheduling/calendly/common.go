// Package calendly holds the shared HTTP client, auth helper, and generic
// list/get/post plumbing used by every scheduling/calendly/* action.
//
// Calendly's API v2 (https://api.calendly.com) is uniform across resources:
// single resources come wrapped in a {"resource": {...}} envelope, collections
// in {"collection": [...], "pagination": {...}} with a next_page_token cursor.
// That regularity lets the request/pagination/result shaping live here once so
// each action package stays thin: read its inputs, call one helper, shape the
// result.
//
// Auth is a Bearer token in the Authorization header. The token input is a
// ConnectionTypeSecret, which accepts both a managed Calendly OAuth credential
// (${credentials.X} — platform-refreshed) and a pasted Personal Access Token
// (${secrets.X}); both resolve to a plain token at run time, so one input
// covers both auth modes.
//
// Calendly addresses resources by full URI (e.g.
// https://api.calendly.com/scheduled_events/<uuid>). Inputs accept either the
// bare UUID or the full URI; helpers normalise in both directions.
package calendly

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// maxResponseBody caps the response body to prevent memory exhaustion.
	// Collection pages of scheduled events can be large, so 8 MB (the
	// airtable/shopify value) rather than 1 MB.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single Calendly call.
	requestTimeout = 30 * time.Second

	// MaxAllPages bounds a "return all" pagination loop so an organisation
	// with a huge history can never spin unbounded requests. On hitting the
	// cap the action returns the outstanding page token so the caller resumes.
	MaxAllPages = 100

	// DefaultPageLimit / MaxPageLimit bound Calendly's per-page count (1-100).
	DefaultPageLimit = 50
	MaxPageLimit     = 100
)

// httpClient is shared across every Calendly action so TCP connections to
// api.calendly.com are pooled and reused rather than re-dialled per call.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// apiBaseURL is a var rather than a const so tests can point every request at
// an httptest server (the same seam idiom as shopify's hostForShop).
var apiBaseURL = "https://api.calendly.com"

// SetBaseURLForTest redirects every request to the given base URL (an
// httptest server) and returns a function that restores the real host. It
// lets action packages in sibling directories exercise Execute end-to-end
// without hitting Calendly. Test-only.
func SetBaseURLForTest(base string) func() {
	prev := apiBaseURL
	apiBaseURL = base
	return func() { apiBaseURL = prev }
}

// AuthInput is the shared credential input every action puts first. Action
// packages re-declare their own literal Inputs arrays (the manifest generator
// AST-parses those); this documents the canonical shape.
var AuthInput = core.Connection{
	Name:        "access_token",
	Type:        core.ConnectionTypeSecret,
	Label:       "Calendly Connection",
	Placeholder: "Calendly credential or Personal Access Token",
	Required:    true,
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
}

// GetAuth resolves the Bearer token from the action's auth input.
func GetAuth(inputs []*core.Connection) (string, error) {
	token, err := RequiredString("access_token", inputs)
	if err != nil {
		return "", fmt.Errorf("authentication required: connect a Calendly credential or supply a Personal Access Token")
	}
	return token, nil
}

// ExecuteAPI performs a REST call against the Calendly API.
// method: GET, POST, DELETE
// path:   resource path including any query string (e.g. "/event_types?user=...")
// body:   optional payload — marshalled to JSON for POST, ignored otherwise
func ExecuteAPI(token, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := apiBaseURL + path

	var bodyReader io.Reader
	if body != nil && method == http.MethodPost {
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
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Calendly API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody}, nil
}

// CheckResponse verifies a 2xx status, decoding Calendly's error envelope
// ({"title": "...", "message": "...", "details": [{parameter, message}]}).
// 429 is surfaced explicitly so the caller understands it hit Calendly's rate
// limit.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("Calendly API rate limit exceeded (429)")
	}

	var env struct {
		Title   string `json:"title"`
		Message string `json:"message"`
		Details []struct {
			Parameter string `json:"parameter"`
			Message   string `json:"message"`
		} `json:"details"`
	}
	if err := json.Unmarshal(resp.Body, &env); err == nil && (env.Title != "" || env.Message != "") {
		msg := env.Message
		if msg == "" {
			msg = env.Title
		}
		var details []string
		for _, d := range env.Details {
			details = append(details, fmt.Sprintf("%s: %s", d.Parameter, d.Message))
		}
		if len(details) > 0 {
			msg += " (" + strings.Join(details, "; ") + ")"
		}
		return fmt.Errorf("Calendly API error (%d): %s", resp.StatusCode, msg)
	}
	return fmt.Errorf("Calendly API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// decode unmarshals a successful response body into a generic map. An empty
// body (e.g. a delete's 204) yields an empty map.
func decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Calendly response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// URI helpers
// ---------------------------------------------------------------------------

// ResourceURI normalises a UUID-or-URI input to the full Calendly URI for a
// resource collection (e.g. ResourceURI("abc", "scheduled_events") →
// "https://api.calendly.com/scheduled_events/abc"). A value that is already a
// URI is returned untouched.
func ResourceURI(idOrURI, collection string) string {
	v := strings.TrimSpace(idOrURI)
	if strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "http://") {
		return v
	}
	return "https://api.calendly.com/" + collection + "/" + v
}

// ExtractUUID normalises a UUID-or-URI input to the bare UUID (the final path
// segment of a Calendly URI).
func ExtractUUID(idOrURI string) string {
	v := strings.TrimSpace(idOrURI)
	v = strings.TrimRight(v, "/")
	if i := strings.LastIndex(v, "/"); i >= 0 {
		return v[i+1:]
	}
	return v
}

// ---------------------------------------------------------------------------
// Current user / scope resolution
// ---------------------------------------------------------------------------

// CurrentUser fetches /users/me and returns the caller's user URI and current
// organisation URI. Most list endpoints require one of the two as a filter, so
// actions resolve them here rather than making the user hunt for URIs.
func CurrentUser(token string) (userURI, orgURI string, err error) {
	resp, err := ExecuteAPI(token, http.MethodGet, "/users/me", nil)
	if err != nil {
		return "", "", err
	}
	if err := CheckResponse(resp); err != nil {
		return "", "", err
	}
	raw, err := decode(resp)
	if err != nil {
		return "", "", err
	}
	resource, _ := raw["resource"].(map[string]interface{})
	if resource == nil {
		return "", "", fmt.Errorf("Calendly /users/me returned no resource")
	}
	userURI, _ = resource["uri"].(string)
	orgURI, _ = resource["current_organization"].(string)
	if userURI == "" || orgURI == "" {
		return "", "", fmt.Errorf("Calendly /users/me response missing uri/current_organization")
	}
	return userURI, orgURI, nil
}

// ScopeInput is the canonical user/organization scope selector used by list
// actions (documented here; action packages re-declare it literally). An
// unset scope means "user".
var ScopeInput = core.Connection{
	Name:  "scope",
	Type:  core.ConnectionTypeString,
	Label: "Scope",
	Options: []core.ConnectionOption{
		{Name: "User", Value: "user"},
		{Name: "Organization", Value: "organization"},
	},
}

// ScopeFilter resolves the "scope" input ("user" default, or "organization")
// into the query parameter Calendly expects, using /users/me for the URIs.
func ScopeFilter(token string, inputs []*core.Connection, q url.Values) error {
	scope := OptionalString("scope", inputs)
	userURI, orgURI, err := CurrentUser(token)
	if err != nil {
		return err
	}
	if scope == "organization" {
		q.Set("organization", orgURI)
	} else {
		q.Set("user", userURI)
	}
	return nil
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

// ClampLimit bounds a requested per-page count to Calendly's 1-100 range,
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

// GetResource GETs a single resource by path (e.g. "/scheduled_events/abc"),
// applying optional query params.
func GetResource(token, path string, q url.Values) (map[string]interface{}, error) {
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	resp, err := ExecuteAPI(token, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// PostResource POSTs a payload to a path and decodes the response.
func PostResource(token, path string, payload interface{}) (map[string]interface{}, error) {
	resp, err := ExecuteAPI(token, http.MethodPost, path, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// DeleteResource DELETEs a single resource by path. Calendly returns 204 with
// no content on success.
func DeleteResource(token, path string) error {
	resp, err := ExecuteAPI(token, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ListResources fetches a collection. When returnAll is false a single page is
// fetched and the next page token (if any) is returned so the caller can
// resume manually. When true it follows pagination.next_page_token until
// exhausted or the MaxAllPages cap. Returns the accumulated items, the
// outstanding next page token (empty when fully drained), the last raw
// response, and the number of pages fetched.
func ListResources(token, resourcePath string, q url.Values, returnAll bool) (items []interface{}, nextPageToken string, lastRaw map[string]interface{}, pages int, err error) {
	// Non-nil so a zero-match list serialises as [] not null — get-many is
	// consumed by Loop nodes that iterate the array.
	items = []interface{}{}
	if q == nil {
		q = url.Values{}
	}

	for {
		path := resourcePath
		if enc := q.Encode(); enc != "" {
			path += "?" + enc
		}
		resp, e := ExecuteAPI(token, http.MethodGet, path, nil)
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
		if arr, ok := raw["collection"].([]interface{}); ok {
			items = append(items, arr...)
		}

		nextPageToken = parseNextPageToken(raw)
		if !returnAll || nextPageToken == "" || pages >= MaxAllPages {
			break
		}
		q.Set("page_token", nextPageToken)
	}
	return items, nextPageToken, lastRaw, pages, nil
}

// parseNextPageToken extracts pagination.next_page_token from a collection
// response. Returns "" when there is no next page.
func parseNextPageToken(raw map[string]interface{}) string {
	pagination, _ := raw["pagination"].(map[string]interface{})
	if pagination == nil {
		return ""
	}
	tok, _ := pagination["next_page_token"].(string)
	return tok
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ResourceResult shapes a single-resource response (get/create/cancel) into
// the standard action output, unwrapping Calendly's {"resource": {...}}
// envelope. id is the resource URI.
func ResourceResult(resp map[string]interface{}, summary string) map[string]interface{} {
	obj, _ := resp["resource"].(map[string]interface{})
	if obj == nil {
		obj = resp
	}
	uri, _ := obj["uri"].(string)
	return map[string]interface{}{
		"id":          uri,
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output:
// results array, count, the outstanding next page token, the raw last page,
// plus the mandatory summary/success/error triple.
func ListResult(items []interface{}, nextPageToken string, lastRaw map[string]interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":         items,
		"count":           len(items),
		"next_page_token": nextPageToken,
		"result":          lastRaw,
		"tool_result":     summary,
		"success":         true,
		"error":           "",
	}
}
