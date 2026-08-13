// Package mailchimp_common holds the shared HTTP client, auth helpers, and the
// request/pagination/result helpers used by every mailchimp/* action.
//
// Mailchimp's Marketing API v3 is datacenter-scoped: the API key ends in a
// datacenter suffix ("<key>-us6") and the base URL is
// https://<dc>.api.mailchimp.com/3.0. We derive the datacenter from the key,
// so a single ConnectionTypeSecret input is all the user supplies (Airtable
// PAT-style). Auth is the header "Authorization: apikey <key>" (Mailchimp also
// accepts HTTP Basic; this matches n8n).
package mailchimp_common

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
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
	// apiPathPrefix is the version segment appended after the datacenter host.
	apiPathPrefix = "/3.0"

	// maxResponseBody caps a single response body to prevent memory exhaustion.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for Mailchimp API calls.
	requestTimeout = 30 * time.Second

	// DefaultPageSize is Mailchimp's maximum page size (count) for list endpoints.
	DefaultPageSize = 1000

	// MaxAllPages bounds the "return all" pagination loop so a misconfigured
	// list can't fetch unboundedly (100 pages * 1000 = 100k items).
	MaxAllPages = 100
)

// httpClient is shared across every Mailchimp action so TCP connections are
// pooled and reused. MaxConnsPerHost is capped at 10 to respect Mailchimp's
// ~10-simultaneous-connections-per-key limit. Matches the connection-reuse
// pattern used by the HubSpot / Airtable integrations.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     10,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AuthInputs is the shared credential input. Action packages declare their own
// literal Inputs arrays (the manifest generator parses those from the AST),
// but this documents the canonical shape they reuse.
var AuthInputs = []core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "Mailchimp API Key",
		Placeholder: "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx-us6",
		Required:    true,
	},
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// datacenter extracts the datacenter suffix from the API key (the segment after
// the final '-', e.g. "...-us6" -> "us6").
func datacenter(apiKey string) (string, error) {
	i := strings.LastIndex(apiKey, "-")
	if i < 0 || i == len(apiKey)-1 {
		return "", fmt.Errorf("invalid Mailchimp API key: expected a datacenter suffix like '-us6'")
	}
	return apiKey[i+1:], nil
}

// baseURL builds the datacenter-scoped API root for the given key.
func baseURL(apiKey string) (string, error) {
	dc, err := datacenter(apiKey)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("https://%s.api.mailchimp.com%s", dc, apiPathPrefix), nil
}

// ExecuteAPI performs a REST call to the Mailchimp API.
// method: GET, POST, PUT, PATCH, DELETE
// path:   absolute path below /3.0, including any query string
//
//	(e.g. "/lists/abc/members?count=10")
//
// body: optional payload — marshalled to JSON for POST/PUT/PATCH, ignored for
// GET/DELETE.
func ExecuteAPI(apiKey, method, path string, body interface{}) (*APIResponse, error) {
	base, err := baseURL(apiKey)
	if err != nil {
		return nil, err
	}
	fullURL := base + path

	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch) {
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

	// n8n uses "Authorization: apikey <key>"; Mailchimp also accepts HTTP Basic.
	req.Header.Set("Authorization", "apikey "+apiKey)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Mailchimp API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// CheckResponse verifies the status code is in the 2xx range, decoding
// Mailchimp's RFC-7807 error envelope ({type,title,status,detail,instance})
// for a human-readable message.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var e struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Status int    `json:"status"`
		Errors []struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.Unmarshal(resp.Body, &e); err == nil && (e.Title != "" || e.Detail != "") {
		msg := strings.TrimSpace(e.Title + ": " + e.Detail)
		for _, fe := range e.Errors {
			msg += fmt.Sprintf(" (%s: %s)", fe.Field, fe.Message)
		}
		return fmt.Errorf("Mailchimp API error (%d): %s", resp.StatusCode, msg)
	}

	return fmt.Errorf("Mailchimp API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// decode unmarshals a successful response body into a generic map (empty body
// — e.g. a 204 — yields an empty map).
func decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Mailchimp response: %w", err)
	}
	return out, nil
}

// Request performs the call and returns the parsed JSON object (execute + check
// + decode).
func Request(apiKey, method, path string, body interface{}) (map[string]interface{}, error) {
	resp, err := ExecuteAPI(apiKey, method, path, body)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// RequestNoContent performs the call and only verifies success (for endpoints
// that return 204 No Content, e.g. campaign send / permanent delete).
func RequestNoContent(apiKey, method, path string, body interface{}) error {
	resp, err := ExecuteAPI(apiKey, method, path, body)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ListAll paginates a Mailchimp collection endpoint, collecting the array under
// prop (e.g. "members", "campaigns", "interests", "lists"). It is bounded by
// MaxAllPages; the executor HTTP layer is context-free (matching HubSpot /
// Airtable), and each page carries the shared client's 30s timeout, so the loop
// always terminates in bounded time and request count.
func ListAll(apiKey, path string, q url.Values, prop string) ([]interface{}, map[string]interface{}, error) {
	if q == nil {
		q = url.Values{}
	}
	var all []interface{}
	var lastRaw map[string]interface{}
	offset := 0
	for pages := 0; pages < MaxAllPages; pages++ {
		q.Set("count", fmt.Sprintf("%d", DefaultPageSize))
		q.Set("offset", fmt.Sprintf("%d", offset))
		raw, err := Request(apiKey, http.MethodGet, path+"?"+q.Encode(), nil)
		if err != nil {
			return nil, nil, err
		}
		lastRaw = raw
		items, _ := raw[prop].([]interface{})
		all = append(all, items...)
		if len(items) < DefaultPageSize {
			break
		}
		offset += DefaultPageSize
	}
	return all, lastRaw, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// GetAPIKey extracts and validates the API key from action inputs.
func GetAPIKey(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("api_key", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", fmt.Errorf("api_key is required")
	}
	return *conn.String(), nil
}

// OptionalString extracts a string input, returning empty string if absent.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return *conn.String()
}

// RequiredString extracts a required string input, erroring if absent.
func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// OptionalInt extracts an integer input; ok is false when absent/empty.
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Number() == nil {
		return 0, false
	}
	return int(*conn.Number()), true
}

// OptionalBool extracts a boolean input, defaulting to false when absent.
func OptionalBool(name string, inputs []*core.Connection) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false
	}
	return *conn.Boolean()
}

// CSVToList splits a comma-separated input into a trimmed, non-empty slice.
func CSVToList(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// SubscriberHash returns Mailchimp's member id for an email: the MD5 hex digest
// of the lowercased address. (n8n passes the raw email, which Mailchimp
// tolerates; hashing is the documented-canonical form and handles casing.)
func SubscriberHash(email string) string {
	sum := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(email))))
	return hex.EncodeToString(sum[:])
}

// MembersPath returns the members collection path for a list/audience.
func MembersPath(listID string) string {
	return "/lists/" + url.PathEscape(listID) + "/members"
}

// MemberPath returns the path for a single member, hashing the email into the
// subscriber-hash path segment.
func MemberPath(listID, email string) string {
	return MembersPath(listID) + "/" + SubscriberHash(email)
}

// CampaignPath returns the path for a single campaign.
func CampaignPath(id string) string {
	return "/campaigns/" + url.PathEscape(id)
}

// BuildMergeFields assembles a Mailchimp merge_fields object (keyed by merge
// tag, e.g. FNAME/LNAME) from a simple key/value list ("merge_fields") plus an
// advanced JSON object ("merge_fields_json") overlaid on top.
func BuildMergeFields(inputs []*core.Connection) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	if conn := core.FindConnection("merge_fields", inputs); conn != nil {
		for _, pair := range conn.KeyValuePairs() {
			if pair.Key != "" {
				out[pair.Key] = pair.Value
			}
		}
	}
	if err := mergeJSONObject("merge_fields_json", inputs, out); err != nil {
		return nil, err
	}
	return out, nil
}

// ParseJSONObject reads an object/text input as a JSON object, returning nil
// when absent/empty.
func ParseJSONObject(name string, inputs []*core.Connection) (map[string]interface{}, error) {
	out := map[string]interface{}{}
	if err := mergeJSONObject(name, inputs, out); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, nil
	}
	return out, nil
}

// mergeJSONObject parses the named input's value (a parsed map or a JSON string)
// into dst. Empty / "{}" / "null" are no-ops.
func mergeJSONObject(name string, inputs []*core.Connection, dst map[string]interface{}) error {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return nil
	}
	switch v := conn.Value.(type) {
	case map[string]interface{}:
		for k, val := range v {
			dst[k] = val
		}
		return nil
	case string:
		return parseJSONInto(name, v, dst)
	default:
		if s := conn.String(); s != nil {
			return parseJSONInto(name, *s, dst)
		}
	}
	return nil
}

func parseJSONInto(name, raw string, dst map[string]interface{}) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return fmt.Errorf("%s must be a JSON object: %w", name, err)
	}
	for k, val := range parsed {
		dst[k] = val
	}
	return nil
}

// summaryWithData appends the JSON-marshalled data to the human summary so the
// AI tool-result (used verbatim by the engine's tool_result fallback chain when
// non-empty) carries BOTH the readable summary AND the structured payload —
// otherwise AI callers only ever see the bare summary and never the data.
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

// ErrorResult returns the standard error output map for a graceful failure.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// MemberResult shapes a single-member response (create/get/update) into the
// standard action output.
func MemberResult(m map[string]interface{}, summary string) map[string]interface{} {
	id, _ := m["id"].(string)
	status, _ := m["status"].(string)
	return map[string]interface{}{
		"id":          id,
		"status":      status,
		"member":      m,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard output.
func ListResult(items []interface{}, total int, raw map[string]interface{}, summary string) map[string]interface{} {
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"total_items": total,
		"result":      raw,
		"tool_result": summaryWithData(summary, items),
		"success":     true,
		"error":       "",
	}
}

// ObjectResult shapes a single-object response (campaign get/replicate) into the
// standard output.
func ObjectResult(obj map[string]interface{}, summary string) map[string]interface{} {
	id, _ := obj["id"].(string)
	return map[string]interface{}{
		"id":          id,
		"result":      obj,
		"tool_result": summaryWithData(summary, obj),
		"success":     true,
		"error":       "",
	}
}

// SuccessResult shapes a no-content success (send/delete) into the standard
// output.
func SuccessResult(summary string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}
