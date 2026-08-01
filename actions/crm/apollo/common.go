// Package apollo_common holds the shared REST client, auth input, input helpers
// and result shapers for every Apollo.io action. It has no Execute function, so
// the manifest generator excludes it from the action registry.
//
// Auth model: Apollo is a paste-an-API-key integration (single key per
// workspace). Each action takes an `api_key` input of ConnectionTypeSecret,
// resolved from an environment secret (${secrets.X}). The key is sent as the
// X-Api-Key header. CRITICAL: the executor runs many tenants' flows
// concurrently, so the key must be bound to a per-call client — never a
// package-level global — to avoid cross-tenant leakage.
package apollo_common

import (
	"bytes"
	"context"
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

// BaseURL is the Apollo API root. A var so tests can point it at an httptest
// server. Apollo's current REST surface lives under /api/v1.
var BaseURL = "https://api.apollo.io/api/v1"

const requestTimeout = 60 * time.Second

// reqContext returns the flow's cancellation context, or a background context
// when flow is nil (unit tests drive Execute with a nil flow).
func reqContext(flow *core.Flow) context.Context {
	if flow == nil {
		return context.Background()
	}
	return flow.GoContext()
}

// Client is an Apollo REST client scoped to a single tenant's API key.
type Client struct {
	apiKey string
	http   *http.Client
}

// NewClient builds a client bound to one workspace's API key.
func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, http: &http.Client{Timeout: requestTimeout}}
}

// Request performs a call and returns the decoded JSON object. A non-2xx
// response yields an *APIError carrying the parsed Apollo message.
func (c *Client) Request(flow *core.Flow, method, path string, query url.Values, body interface{}) (map[string]interface{}, error) {
	u := BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(reqContext(flow), method, u, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Status: resp.StatusCode, Body: respBody}
	}

	var decoded map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &decoded); err != nil {
			return nil, fmt.Errorf("unable to parse Apollo response: %w", err)
		}
	}
	return decoded, nil
}

// Post/Patch/Get are thin wrappers over Request for readability in actions.
func (c *Client) Post(flow *core.Flow, path string, body interface{}) (map[string]interface{}, error) {
	return c.Request(flow, http.MethodPost, path, nil, body)
}

func (c *Client) Patch(flow *core.Flow, path string, body interface{}) (map[string]interface{}, error) {
	return c.Request(flow, http.MethodPatch, path, nil, body)
}

func (c *Client) Get(flow *core.Flow, path string, query url.Values) (map[string]interface{}, error) {
	return c.Request(flow, http.MethodGet, path, query, nil)
}

// APIError is a non-2xx Apollo response, parsed for a human-readable message.
type APIError struct {
	Status int
	Body   []byte
}

func (e *APIError) Error() string { return e.Message() }

// Message pulls the best available text from Apollo's varied error shapes
// ({"error":"…"}, {"error_message":"…"}, {"errors":["…"]}), falling back to the
// raw body / status code.
func (e *APIError) Message() string {
	var parsed map[string]interface{}
	if json.Unmarshal(e.Body, &parsed) == nil {
		for _, k := range []string{"error", "error_message", "message"} {
			if s, ok := parsed[k].(string); ok && s != "" {
				return s
			}
		}
		if arr, ok := parsed["errors"].([]interface{}); ok && len(arr) > 0 {
			parts := make([]string, 0, len(arr))
			for _, a := range arr {
				if s, ok := a.(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "; ")
			}
		}
	}
	if s := strings.TrimSpace(string(e.Body)); s != "" {
		return fmt.Sprintf("Apollo API error (HTTP %d): %s", e.Status, s)
	}
	return fmt.Sprintf("Apollo API error (HTTP %d)", e.Status)
}

// AuthInputs is the shared credential input every Apollo action embeds first.
var AuthInputs = []core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
}

// NOTE: outputs are declared as inline literals inside each action (NOT a shared
// var), because the manifest generator only resolves inline composite literals —
// a cross-package Outputs reference yields empty manifest outputs and the editor
// draws no source handle. See the QBO/Xero note in CLAUDE.md.

// --- input helpers ---

func GetAPIKey(inputs []*core.Connection) (string, error) {
	key, err := RequiredString("api_key", inputs)
	if err != nil {
		return "", fmt.Errorf("an Apollo API key is required")
	}
	if strings.HasPrefix(key, "${") {
		return "", fmt.Errorf("the Apollo API key did not resolve — set an environment secret and reference it as ${secrets.X}")
	}
	return key, nil
}

func RequiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return *c.String(), nil
}

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func OptionalInt(name string, inputs []*core.Connection) *int64 {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	return c.Number()
}

func OptionalBool(name string, inputs []*core.Connection) *bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	return c.Boolean()
}

// StringList splits a comma-separated input (e.g. contact_ids) into a trimmed
// slice, dropping blanks. Returns nil when the input is absent/blank.
func StringList(name string, inputs []*core.Connection) []string {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// --- body setters (assign into the request body only when present) ---

func SetString(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalString(name, inputs); v != "" {
		body[field] = v
	}
}

func SetInt(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalInt(name, inputs); v != nil {
		body[field] = *v
	}
}

func SetBool(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalBool(name, inputs); v != nil {
		body[field] = *v
	}
}

// SetList assigns a comma-separated input as a JSON array when non-empty.
func SetList(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := StringList(name, inputs); len(v) > 0 {
		body[field] = v
	}
}

// RangeList splits an input into range strings on SEMICOLONS and newlines only,
// preserving the comma INSIDE each range. Apollo's headcount filter expects an
// array of "min,max" strings (e.g. ["50,5000"]), so a plain comma-split would
// wrongly break "50,5000" into two elements. Returns nil when absent/blank.
func RangeList(name string, inputs []*core.Connection) []string {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == '\n' || r == '\r' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SetRangeList assigns a semicolon-separated input as a JSON array of range
// strings (each "min,max") when non-empty.
func SetRangeList(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := RangeList(name, inputs); len(v) > 0 {
		body[field] = v
	}
}

// SetNumberValue parses a decimal input (e.g. deal amount) as a JSON number.
func SetNumberValue(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return
	}
	if f, err := strconv.ParseFloat(strings.Map(stripMoneyRune, raw), 64); err == nil {
		body[field] = f
	}
}

func stripMoneyRune(r rune) rune {
	switch r {
	case '£', '$', '€', '¥', ',', ' ', '\t':
		return -1
	}
	return r
}

// ParseJSONObject reads a text input as a JSON object (advanced `fields`
// override). Returns nil when absent so nothing is merged.
func ParseJSONObject(name string, inputs []*core.Connection) (map[string]interface{}, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", name, err)
	}
	return m, nil
}

// ParseJSONArray reads a text input as a JSON array (e.g. bulk match details).
// Returns nil when absent.
func ParseJSONArray(name string, inputs []*core.Connection) ([]interface{}, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var a []interface{}
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, fmt.Errorf("invalid JSON array in %s: %w", name, err)
	}
	return a, nil
}

// MergeFields overlays extra JSON fields onto a body map (advanced overrides).
func MergeFields(body, extra map[string]interface{}) {
	for k, v := range extra {
		body[k] = v
	}
}

// --- response extraction ---

// Obj pulls a named object out of an Apollo response ({"person":{…}}).
func Obj(resp map[string]interface{}, key string) map[string]interface{} {
	obj, _ := resp[key].(map[string]interface{})
	return obj
}

// Arr pulls a named array of objects out of a response ({"people":[…]}).
func Arr(resp map[string]interface{}, key string) []map[string]interface{} {
	arr, ok := resp[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

// IDOf reads the "id" field of an Apollo object.
func IDOf(obj map[string]interface{}) string {
	if obj == nil {
		return ""
	}
	id, _ := obj["id"].(string)
	return id
}

// --- result shapers ---

// ObjectResult wraps a single Apollo object result for downstream nodes.
func ObjectResult(id string, obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	if id == "" {
		id = IDOf(obj)
	}
	return map[string]interface{}{
		"tool_result": summary,
		"id":          id,
		"result":      obj,
		"success":     true,
		"error":       "",
	}
}

// ListResult wraps a collection of Apollo objects.
func ListResult(items []map[string]interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []map[string]interface{}{}
	}
	return map[string]interface{}{
		"tool_result": summary,
		"results":     items,
		"count":       len(items),
		"success":     true,
		"error":       "",
	}
}

// ErrorResult is a graceful failure — success=false, not a node error — so an
// invalid parameter or rate limit can be handled within the flow.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// MapError converts any request error into a graceful ErrorResult. A 429 is
// surfaced verbatim so the flow author sees Apollo's rate-limit message.
func MapError(err error) map[string]interface{} {
	if ae, ok := err.(*APIError); ok {
		return ErrorResult(ae.Message())
	}
	return ErrorResult(err.Error())
}
