// Package azureaisearch holds the shared HTTP client, auth resolution, input
// helpers and result shaping for every vectordatabase/azureaisearch/* action.
//
// Azure AI Search is a REST-only surface: index management and the document
// data plane share one host — https://{service}.search.windows.net — one auth
// header and one mandatory ?api-version query parameter on every call. Errors
// come back as the standard Azure envelope {"error":{"code","message"}}.
//
// Three provider quirks shape this file:
//
//   - Auth is a bare "api-key" header, NOT Authorization: Bearer. The key must
//     be an ADMIN key for anything that writes (index create/delete, document
//     upload); a query key only covers reads.
//   - GET /indexes/{name}/docs/$count returns a text/plain integer — possibly
//     prefixed with a UTF-8 BOM — not JSON. See ParseCount.
//   - Collection responses wrap the payload in an OData "value" array, and
//     search results carry the total as "@odata.count" (only when the request
//     asked for it with "count": true).
//
// The REST idioms (pooled client, APIResponse, CheckResponse, ErrorResult,
// ResourceResult) follow actions/cms/wordpress/common.go; the category.go
// shape and output conventions follow the sibling vectordatabase/pgvector.
package azureaisearch

import (
	"bytes"
	"context"
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
	// DefaultAPIVersion is sent when the api_version input is blank. 2024-07-01
	// is the current GA data-plane version (vector search + semantic ranking
	// are GA in it); pinning a default keeps the UI clean while letting an
	// operator on a sovereign cloud or preview feature override per-node.
	DefaultAPIVersion = "2024-07-01"

	// maxResponseBody caps response reads. Search hits with large documents
	// can be bulky, so 8 MB (the wordpress/woocommerce value).
	maxResponseBody = 8 << 20 // 8 MB

	requestTimeout = 60 * time.Second

	// DefaultTop / MaxTop bound the search "top" page size. The service caps a
	// single page at 1000 results.
	DefaultTop = 50
	MaxTop     = 1000
)

// httpClient is shared across every Azure AI Search action so TLS connections
// to the service are pooled and reused rather than re-dialled per call.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AuthInputs is the canonical credential block. Action packages re-declare
// their own literal Inputs arrays (the manifest generator AST-parses those
// literals, so it cannot see through a shared variable), but every action puts
// these four first, in this order — azureaisearch_inputs_drift_test.go is the
// enforcement.
//
// The names are reserved: core.FindConnection returns the FIRST input whose
// name matches, and the credential block is declared first, so a resource
// field reusing one of these names would silently read the credential instead.
var AuthInputs = []core.Connection{
	{
		Name:        "service_name",
		Type:        core.ConnectionTypeString,
		Label:       "Service Name",
		Placeholder: "my-search-service — the {name} in https://{name}.search.windows.net",
	},
	{
		Name:        "endpoint",
		Type:        core.ConnectionTypeString,
		Label:       "Custom Endpoint",
		Placeholder: "https://my-search-service.search.windows.net — overrides Service Name (sovereign clouds, private endpoints)",
	},
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Key",
		Placeholder: "Admin key from Settings ▸ Keys — a query key only covers reads",
		Required:    true,
	},
	{
		Name:        "api_version",
		Type:        core.ConnectionTypeString,
		Label:       "API Version",
		Placeholder: "2024-07-01 (default)",
	},
}

// Auth is the resolved connection: the service base URL (scheme + host, no
// trailing slash), the admin/query key, and the api-version every call carries.
type Auth struct {
	BaseURL    string
	APIKey     string
	APIVersion string
}

// APIResponse wraps the HTTP response for consistent handling. Headers is
// carried for completeness (ETags on index responses).
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// serviceNameRe validates the search service name before it is interpolated
// into a hostname — a metacharacter here would redirect the request. Azure
// service names are 2-60 lowercase letters/digits/dashes; the check is looser
// (case, length 90) so it rejects injection without fighting edge cases.
var serviceNameRe = regexp.MustCompile(`^[a-zA-Z0-9-]{1,90}$`)

// GetAuth resolves the endpoint, API key and api-version from the action's
// auth inputs. A missing key or unusable endpoint is a hard failure (zero Auth
// + real error) — there is nothing to attempt without them. The Custom
// Endpoint wins over Service Name when both are set.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	key, err := RequiredString("api_key", inputs)
	if err != nil {
		return Auth{}, err
	}

	base := ""
	if endpoint := OptionalString("endpoint", inputs); endpoint != "" {
		u, err := url.Parse(endpoint)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Auth{}, fmt.Errorf("endpoint must be an http(s) URL, e.g. https://my-search-service.search.windows.net")
		}
		base = strings.TrimRight(endpoint, "/")
	} else {
		name := OptionalString("service_name", inputs)
		if name == "" {
			return Auth{}, fmt.Errorf("service_name is required (or set a Custom Endpoint)")
		}
		if !serviceNameRe.MatchString(name) {
			return Auth{}, fmt.Errorf("service_name %q is not a valid search service name (letters, digits and dashes only)", name)
		}
		base = "https://" + name + ".search.windows.net"
	}

	version := OptionalString("api_version", inputs)
	if version == "" {
		version = DefaultAPIVersion
	}

	return Auth{BaseURL: base, APIKey: key, APIVersion: version}, nil
}

// reqContext returns the flow's cancellation context, or a background context
// when flow is nil (unit tests drive Execute with a nil flow).
func reqContext(flow *core.Flow) context.Context {
	if flow == nil {
		return context.Background()
	}
	return flow.GoContext()
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// ExecuteAPI performs a REST call against the search service.
// path:  resource path (e.g. "/indexes/products/docs/search")
// query: optional extra query params; ?api-version={auth} is always appended
// body:  optional payload, marshalled to JSON
// extra: optional extra headers (If-None-Match on conditional index creates)
func ExecuteAPI(flow *core.Flow, a Auth, method, path string, query url.Values, body interface{}, extra http.Header) (*APIResponse, error) {
	q := url.Values{}
	for k, vs := range query {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	q.Set("api-version", a.APIVersion)
	fullURL := a.BaseURL + path + "?" + q.Encode()

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(reqContext(flow), method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", Redact(a, err.Error()))
	}
	req.Header.Set("api-key", a.APIKey)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, vs := range extra {
		for _, v := range vs {
			req.Header.Set(k, v)
		}
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Azure AI Search request failed: %s", Redact(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %s", Redact(a, err.Error()))
	}
	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// Redact masks the API key wherever it appears in an error string. The key
// travels in a header (not the URL), so a leak is unlikely — but a proxy or
// wrapped error could echo it, so it is scrubbed defensively.
func Redact(a Auth, msg string) string {
	if a.APIKey == "" {
		return msg
	}
	return strings.ReplaceAll(msg, a.APIKey, "********")
}

// CheckResponse verifies a 2xx status, decoding the Azure error envelope
// {"error":{"code","message"}}. The code is prefixed when present so an
// unfamiliar error is greppable against the Azure docs.
func CheckResponse(a Auth, resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &env); err == nil && env.Error.Message != "" {
		msg := env.Error.Message
		if env.Error.Code != "" {
			msg = env.Error.Code + ": " + msg
		}
		return fmt.Errorf("Azure AI Search error (%d): %s", resp.StatusCode, Redact(a, msg))
	}
	body := string(resp.Body)
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Errorf("Azure AI Search error (%d): %s", resp.StatusCode, Redact(a, body))
}

// Decode unmarshals a successful single-object body into a generic map. An
// empty body (204s) decodes to an empty map.
func Decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Azure AI Search response: %w", err)
	}
	return out, nil
}

// DecodeValue unmarshals a collection body and unwraps the OData "value"
// array. A non-nil empty slice is returned when the array is absent/empty so
// a zero-match list serialises as [] not null (get-many feeds Loop nodes).
func DecodeValue(resp *APIResponse) ([]interface{}, error) {
	obj, err := Decode(resp)
	if err != nil {
		return nil, err
	}
	items, ok := obj["value"].([]interface{})
	if !ok {
		return []interface{}{}, nil
	}
	return items, nil
}

// ParseCount parses the /docs/$count reply: a text/plain integer, possibly
// prefixed with a UTF-8 BOM (the service emits one).
func ParseCount(body []byte) (int64, error) {
	s := strings.TrimSpace(strings.TrimPrefix(string(body), "\ufeff"))
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("could not parse document count %q", s)
	}
	return n, nil
}

// ParseIndexingStatuses decodes the POST /docs/index reply — a "value" array
// of per-document results {key, status, errorMessage, statusCode} — and
// collects the keys that failed with their reasons. The endpoint answers 200
// when every document succeeded and 207 (also 2xx) on partial failure, so the
// status array, not the HTTP status, is the real verdict.
func ParseIndexingStatuses(resp *APIResponse) (statuses []interface{}, failed []string, err error) {
	statuses, err = DecodeValue(resp)
	if err != nil {
		return nil, nil, err
	}
	for _, s := range statuses {
		obj, ok := s.(map[string]interface{})
		if !ok {
			continue
		}
		if ok, _ := obj["status"].(bool); ok {
			continue
		}
		key, _ := obj["key"].(string)
		if msg, _ := obj["errorMessage"].(string); msg != "" {
			failed = append(failed, fmt.Sprintf("%s (%s)", key, msg))
		} else {
			failed = append(failed, key)
		}
	}
	return statuses, failed, nil
}

// SummariseFailedKeys renders the failed-key list for an error message,
// truncating so one bad batch doesn't produce a megabyte of tool_result.
func SummariseFailedKeys(failed []string) string {
	const maxListed = 10
	if len(failed) <= maxListed {
		return strings.Join(failed, ", ")
	}
	return strings.Join(failed[:maxListed], ", ") + fmt.Sprintf(", … and %d more", len(failed)-maxListed)
}

// EscapeDocKey renders a document key for the /docs('{key}') lookup path.
// Single quotes are doubled per OData string-literal rules, then the segment
// is percent-encoded.
func EscapeDocKey(key string) string {
	return url.PathEscape(strings.ReplaceAll(key, "'", "''"))
}

// IndexPath returns the /indexes/{name} path with the name segment-encoded
// (index names are lowercase letters/digits/dashes, but encoding defensively
// keeps a bad name from rewriting the path).
func IndexPath(name string) string {
	return "/indexes/" + url.PathEscape(name)
}

// ---------------------------------------------------------------------------
// Input helpers (mirrored from actions/cms/wordpress/common.go)
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

// SetIfPresent adds an optional string field to a request body only when the
// input was provided, so unset fields are omitted.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// ClampLimit bounds a requested search page size to the service's 1-1000
// range, falling back to DefaultTop when unset.
func ClampLimit(limit int, set bool) int {
	if !set || limit <= 0 {
		return DefaultTop
	}
	if limit > MaxTop {
		return MaxTop
	}
	return limit
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

// ResourceResult shapes a single-object response into the standard action
// output. Azure AI Search objects have no uniform "id" property (indexes key
// on "name", documents on a per-index key field), so the caller supplies it.
func ResourceResult(id string, obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          id,
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output.
// count is separate from len(items) because a search response's @odata.count
// is the TOTAL match count, of which items is one page.
func ListResult(items []interface{}, count int, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       count,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}
