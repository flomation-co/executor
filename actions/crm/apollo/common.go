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

// toolResultWithData renders the AI-facing tool_result as the summary followed
// by the JSON payload. This is load-bearing: when an action is invoked as an AI
// tool the agent receives ONLY tool_result (the `result`/`results` outputs are
// for downstream flow nodes), so a summary-only tool_result — e.g. "Found 10
// people" — starves the agent of the actual records. The flow engine truncates
// oversized tool results by token budget, so including the full payload is safe.
func toolResultWithData(summary string, data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil || len(b) == 0 || string(b) == "null" || string(b) == "{}" || string(b) == "[]" {
		return summary
	}
	return summary + ":\n" + string(b)
}

// ObjectResult wraps a single Apollo object result for downstream nodes. The
// object is embedded in tool_result so an AI caller gets the fields, not just
// the summary.
func ObjectResult(id string, obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	if id == "" {
		id = IDOf(obj)
	}
	return map[string]interface{}{
		"tool_result": toolResultWithData(summary, obj),
		"id":          id,
		"result":      obj,
		"success":     true,
		"error":       "",
	}
}

// ListResult wraps a collection of Apollo objects. The records are embedded in
// tool_result so an AI caller receives the actual data (names, emails, …), not
// just a count.
func ListResult(items []map[string]interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []map[string]interface{}{}
	}
	return map[string]interface{}{
		"tool_result": toolResultWithData(summary, items),
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

// ── Query-parameter builders ────────────────────────────────────────────────
//
// CRITICAL: Apollo's search endpoints (people, companies, contacts, accounts,
// sequences, emailer messages) read their filters from the URL QUERY STRING,
// not the JSON request body — array filters use bracket notation
// (key[]=a&key[]=b). Sending filters in the body makes Apollo silently ignore
// them and return a generic, unscoped list. Every search action must build its
// filters with these helpers and POST via PostQuery.

// PostQuery POSTs to path with the filters in the query string and an empty JSON
// body (which preserves the application/json Content-Type Apollo expects).
func (c *Client) PostQuery(flow *core.Flow, path string, query url.Values) (map[string]interface{}, error) {
	return c.Request(flow, http.MethodPost, path, query, map[string]interface{}{})
}

// AddQueryString sets a scalar query param from a string input, when non-empty.
func AddQueryString(q url.Values, key, name string, inputs []*core.Connection) {
	if v := strings.TrimSpace(OptionalString(name, inputs)); v != "" {
		q.Set(key, v)
	}
}

// AddQueryList adds an array input as repeated bracketed params: key[]=a&key[]=b.
func AddQueryList(q url.Values, key, name string, inputs []*core.Connection) {
	for _, v := range StringList(name, inputs) {
		if t := strings.TrimSpace(v); t != "" {
			q.Add(key+"[]", t)
		}
	}
}

// AddQueryRangeList adds a range input (e.g. "50,5000") as bracketed params,
// preserving the comma inside each range.
func AddQueryRangeList(q url.Values, key, name string, inputs []*core.Connection) {
	for _, v := range RangeList(name, inputs) {
		if t := strings.TrimSpace(v); t != "" {
			q.Add(key+"[]", t)
		}
	}
}

// AddQueryInt sets an integer query param when the input is present.
func AddQueryInt(q url.Values, key, name string, inputs []*core.Connection) {
	if p := OptionalInt(name, inputs); p != nil {
		q.Set(key, strconv.FormatInt(*p, 10))
	}
}

// AddQueryBool sets a boolean query param when the input is present.
func AddQueryBool(q url.Values, key, name string, inputs []*core.Connection) {
	if b := OptionalBool(name, inputs); b != nil {
		q.Set(key, strconv.FormatBool(*b))
	}
}

// AddQueryFromMap flattens an arbitrary JSON object (a `fields` override) into
// query params: scalars as key=value, arrays as key[]=item.
func AddQueryFromMap(q url.Values, m map[string]interface{}) {
	for k, v := range m {
		switch vv := v.(type) {
		case nil:
		case string:
			q.Set(k, vv)
		case bool:
			q.Set(k, strconv.FormatBool(vv))
		case float64:
			q.Set(k, strconv.FormatFloat(vv, 'f', -1, 64))
		case []interface{}:
			for _, item := range vv {
				q.Add(k+"[]", fmt.Sprint(item))
			}
		default:
			q.Set(k, fmt.Sprint(vv))
		}
	}
}

// ── Plan-gated / obfuscated data detection ──────────────────────────────────
//
// On limited Apollo plans, People Search and enrichment return records whose
// personal data is WITHHELD: the surname comes back only as
// `last_name_obfuscated` ("Mc***y") and email/city/phone are replaced by
// `has_email` / `has_city` / `has_direct_phone` boolean flags with no actual
// value. The results look plausible but are not verifiable or contactable, and
// revealing them needs a plan with API access and enrichment credits. These
// helpers detect that so an action can warn loudly rather than let an agent
// present masked people as a confident cohort.

// IsGatedRecord reports whether an Apollo person record has plan-gated personal
// data (an obfuscated surname, or a has_* flag set while the real value is
// absent/empty).
func IsGatedRecord(m map[string]interface{}) bool {
	if m == nil {
		return false
	}
	if s, ok := m["last_name_obfuscated"].(string); ok && strings.TrimSpace(s) != "" {
		return true
	}
	if truthyFlag(m["has_email"]) && emptyValue(m["email"]) {
		return true
	}
	if truthyFlag(m["has_city"]) && emptyValue(m["city"]) {
		return true
	}
	return false
}

// GatePrefix returns summary with a loud data-gating warning prepended when any
// of the records are plan-gated; otherwise it returns summary unchanged. The
// warning is written into tool_result so an AI/agent caller cannot miss it.
func GatePrefix(summary string, records []map[string]interface{}) string {
	gated := 0
	for _, m := range records {
		if IsGatedRecord(m) {
			gated++
		}
	}
	if gated == 0 {
		return summary
	}
	warn := fmt.Sprintf("WARNING - APOLLO DATA GATED: %d of %d result(s) have personal data withheld by this API key's Apollo plan (surnames obfuscated as last_name_obfuscated; email/city/phone hidden behind has_* flags). These people CANNOT be verified or contacted from this key - an Apollo plan with API access and available credits is required to reveal (unlock) them. Do NOT present these as confirmed contacts.", gated, len(records))
	if strings.TrimSpace(summary) == "" {
		return warn
	}
	return warn + "\n\n" + summary
}

// truthyFlag reads Apollo's has_* flags, which arrive as either a bool or a
// string ("true"/"Yes").
func truthyFlag(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "yes"
	}
	return false
}

func emptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}
