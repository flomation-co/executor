// Package heygen_common holds the shared REST client, auth input and helpers
// for every HeyGen action. It has no Execute function, so the manifest
// generator excludes it from the action registry.
//
// Auth model: HeyGen is a paste-an-API-key integration. Each action takes an
// `api_key` input of ConnectionTypeSecret, resolved from an environment secret
// (${secrets.X}) and sent as the X-Api-Key header. CRITICAL: the executor runs
// many tenants' flows concurrently, so the key is bound to a per-call client —
// never a package-level global — to avoid cross-tenant leakage.
//
// API: HeyGen v3 (https://api.heygen.com/v3/*). Responses wrap their payload in
// a top-level "data" object.
package heygen_common

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

// BaseURL is the HeyGen API root. A var so tests can point it at an httptest
// server.
var BaseURL = "https://api.heygen.com"

const requestTimeout = 60 * time.Second

func reqContext(flow *core.Flow) context.Context {
	if flow == nil {
		return context.Background()
	}
	return flow.GoContext()
}

// Client is a HeyGen REST client scoped to a single tenant's API key.
type Client struct {
	apiKey string
	http   *http.Client
}

// NewClient builds a client bound to one workspace's API key.
func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, http: &http.Client{Timeout: requestTimeout}}
}

// Request performs a call and returns the decoded JSON object. A non-2xx
// response yields an *APIError carrying the parsed HeyGen message.
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
			return nil, fmt.Errorf("unable to parse HeyGen response: %w", err)
		}
	}
	return decoded, nil
}

// Get/Post are thin wrappers over Request for readability in actions.
func (c *Client) Get(flow *core.Flow, path string, query url.Values) (map[string]interface{}, error) {
	return c.Request(flow, http.MethodGet, path, query, nil)
}

func (c *Client) Post(flow *core.Flow, path string, body interface{}) (map[string]interface{}, error) {
	return c.Request(flow, http.MethodPost, path, nil, body)
}

// APIError is a non-2xx HeyGen response, parsed for a human-readable message.
type APIError struct {
	Status int
	Body   []byte
}

func (e *APIError) Error() string { return e.Message() }

// Message pulls the best available text from HeyGen's error shapes
// ({"error":{"code","message"}}, {"message":"…"}, {"error":"…"}), falling back
// to the raw body / status code. A 429 is surfaced verbatim so rate limiting is
// obvious to the caller.
func (e *APIError) Message() string {
	var parsed map[string]interface{}
	if json.Unmarshal(e.Body, &parsed) == nil {
		if errObj, ok := parsed["error"].(map[string]interface{}); ok {
			msg, _ := errObj["message"].(string)
			code, _ := errObj["code"].(string)
			switch {
			case msg != "" && code != "":
				return fmt.Sprintf("%s (%s)", msg, code)
			case msg != "":
				return msg
			case code != "":
				return code
			}
		}
		for _, k := range []string{"message", "error", "msg"} {
			if s, ok := parsed[k].(string); ok && s != "" {
				return s
			}
		}
	}
	if s := strings.TrimSpace(string(e.Body)); s != "" {
		return fmt.Sprintf("HeyGen API error (HTTP %d): %s", e.Status, s)
	}
	return fmt.Sprintf("HeyGen API error (HTTP %d)", e.Status)
}

// AuthInputs is the shared credential input every HeyGen action embeds first.
var AuthInputs = []core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HeyGen API Key", Placeholder: "${secrets.HeyGenApiKey}", Required: true},
}

// NOTE: outputs are declared as inline literals inside each action (NOT a shared
// var) because the manifest generator only resolves inline composite literals.

// --- input helpers ---

func GetAPIKey(inputs []*core.Connection) (string, error) {
	key := OptionalString("api_key", inputs)
	if key == "" {
		return "", fmt.Errorf("a HeyGen API key is required")
	}
	if strings.HasPrefix(key, "${") {
		return "", fmt.Errorf("the HeyGen API key did not resolve — set an environment secret and reference it as ${secrets.X}")
	}
	return key, nil
}

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return strings.TrimSpace(*c.String())
}

func OptionalInt(name string, inputs []*core.Connection) *int64 {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	return c.Number()
}

// OptionalBool extracts an optional boolean input, nil when absent.
func OptionalBool(name string, inputs []*core.Connection) *bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	return c.Boolean()
}

// OptionalFloat parses a decimal string input (e.g. voice speed) as a float.
// Returns nil when absent or unparseable.
func OptionalFloat(name string, inputs []*core.Connection) *float64 {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		return &f
	}
	return nil
}

// ParseJSONObject reads a text input as a JSON object (advanced override).
// Returns nil when absent so nothing is merged.
func ParseJSONObject(name string, inputs []*core.Connection) (map[string]interface{}, error) {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", name, err)
	}
	return m, nil
}

// --- body setters (assign into the request body only when present) ---

func SetString(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalString(name, inputs); v != "" {
		body[field] = v
	}
}

// --- response extraction ---

// DataObj returns the top-level "data" object HeyGen wraps most responses in.
func DataObj(resp map[string]interface{}) map[string]interface{} {
	if d, ok := resp["data"].(map[string]interface{}); ok {
		return d
	}
	return nil
}

// ExtractList pulls a list of records out of a HeyGen list response, tolerating
// the shape variance across endpoints: data may be an array directly, or an
// object holding one of the given candidate array keys (e.g. "voices",
// "avatars", "looks", "list").
func ExtractList(resp map[string]interface{}, candidateKeys ...string) []map[string]interface{} {
	take := func(v interface{}) []map[string]interface{} {
		arr, ok := v.([]interface{})
		if !ok {
			return nil
		}
		out := make([]map[string]interface{}, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}
	if arr := take(resp["data"]); arr != nil {
		return arr
	}
	if d := DataObj(resp); d != nil {
		for _, k := range candidateKeys {
			if arr := take(d[k]); arr != nil {
				return arr
			}
		}
	}
	for _, k := range candidateKeys {
		if arr := take(resp[k]); arr != nil {
			return arr
		}
	}
	return []map[string]interface{}{}
}

// Str reads a string field from a map, tolerating a nil map.
func Str(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if s, ok := m[key].(string); ok {
		return s
	}
	return ""
}

// --- result shapers ---

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

func MapError(err error) map[string]interface{} {
	if ae, ok := err.(*APIError); ok {
		return ErrorResult(ae.Message())
	}
	return ErrorResult(err.Error())
}

// Result builds a success output map from a tool_result summary plus extra
// named outputs. success/error are set for the caller.
func Result(toolResult string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{
		"tool_result": toolResult,
		"success":     true,
		"error":       "",
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}
