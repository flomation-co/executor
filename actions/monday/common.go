// Package monday_common holds the shared GraphQL client, auth helpers, and
// result shapers used by every monday/* action.
//
// Monday.com is unlike every other provider node so far: it exposes a single
// GraphQL endpoint (https://api.monday.com/v2) rather than a REST surface. Every
// operation is a GraphQL query or mutation POSTed as {"query": "...",
// "variables": {...}}, and the response comes back as {"data": {...}, "errors":
// [...]}. That shape lives here once so each action package only has to build its
// query string and read the one field it cares about out of `data`.
//
// Four things shape this file:
//
//   - Auth is an API token (v2) sent as an HTTP Bearer header, plus a fixed
//     "API-Version" header. The token is a ConnectionTypeSecret, scrubbed from
//     any error string. The host is FIXED (api.monday.com), never
//     caller-supplied, so there is no SSRF surface.
//
//   - GraphQL returns HTTP 200 even for errors — the failure is carried in the
//     top-level "errors" array (or an "error_message"). CheckErrors surfaces
//     those, so a 200 with errors is never mistaken for success.
//
//   - IDs come back as strings ("123456789"). ResourceResult reads obj["id"].
//
//   - "JSON!" GraphQL arguments (a column value, a create's column_values, a
//     column's defaults) are passed as a JSON-ENCODED STRING variable — a Monday
//     quirk. Actions validate the operator's JSON, then pass the string through.
package monday_common

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// APIBase is the fixed Monday.com GraphQL endpoint. It is a constant (never
	// caller-supplied), so there is no SSRF surface.
	APIBase = "https://api.monday.com/v2"

	// APIVersion pins the Monday API version (they date-version the schema).
	APIVersion = "2023-10"

	maxResponseBody = 8 << 20 // 8 MB
	requestTimeout  = 30 * time.Second

	// DefaultPageLimit / MaxPageLimit bound a single list page.
	DefaultPageLimit = 50
	MaxPageLimit     = 100
)

var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AuthInputs is the shared credential shape. Action packages re-declare their own
// literal Inputs arrays (the manifest generator AST-parses those), but this
// documents the canonical set every action puts first. The secret is named
// api_token (not "token") so a resource field never collides with it via
// core.FindConnection's first-match behaviour.
var AuthInputs = []core.Connection{
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Token",
		Placeholder: "Your Monday.com API token (avatar ▸ Developers ▸ My Access Tokens)",
		Required:    true,
	},
}

// Auth is the resolved credential.
type Auth struct {
	Token string
}

// GetAuth resolves the API token from the action's auth inputs.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	token, err := RequiredString("api_token", inputs)
	if err != nil {
		return Auth{}, err
	}
	return Auth{Token: token}, nil
}

// apiBase is a var so SetBaseForTest can point every request at an httptest
// server.
var apiBase = APIBase

// SetBaseForTest redirects every request to the given base URL and returns a
// restore function. Test-only.
func SetBaseForTest(base string) func() {
	prev := apiBase
	apiBase = strings.TrimRight(base, "/")
	return func() { apiBase = prev }
}

// ---------------------------------------------------------------------------
// GraphQL transport
// ---------------------------------------------------------------------------

// graphResponse is the envelope every Monday response shares.
type graphResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
	// Some auth/complexity failures use these instead of the GraphQL errors array.
	ErrorMessage string `json:"error_message"`
	ErrorCode    string `json:"error_code"`
}

// GraphQL POSTs a query/mutation with variables and returns the decoded `data`
// object. A GraphQL-level error (200 + errors array, or an error_message) is
// turned into a Go error with the token scrubbed.
func GraphQL(a Auth, query string, variables map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{"query": query}
	if variables != nil {
		payload["variables"] = variables
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal GraphQL request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiBase, bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", redactAuth(a, err.Error()))
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("API-Version", APIVersion)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Monday.com API request failed: %s", redactAuth(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// A non-2xx with no parseable body (e.g. 401/429) still needs surfacing.
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("Monday.com rejected the request as unauthorised — check the API Token")
	}

	var env graphResponse
	if err := json.Unmarshal(body, &env); err != nil {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return nil, fmt.Errorf("Monday.com API error (%d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("failed to parse Monday.com response: %w", err)
	}

	if len(env.Errors) > 0 {
		msgs := make([]string, 0, len(env.Errors))
		for _, e := range env.Errors {
			if e.Message != "" {
				msgs = append(msgs, e.Message)
			}
		}
		return nil, fmt.Errorf("Monday.com GraphQL error: %s", redactAuth(a, strings.Join(msgs, "; ")))
	}
	if env.ErrorMessage != "" {
		return nil, fmt.Errorf("Monday.com error: %s", redactAuth(a, env.ErrorMessage))
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Monday.com API error (%d)", resp.StatusCode)
	}

	out := map[string]interface{}{}
	if len(env.Data) > 0 {
		if err := json.Unmarshal(env.Data, &out); err != nil {
			return nil, fmt.Errorf("failed to parse Monday.com response data: %w", err)
		}
	}
	return out, nil
}

// redactAuth removes the API token from an error message (defensive — it travels
// in the Authorization header, not the body).
func redactAuth(a Auth, msg string) string {
	if a.Token != "" {
		msg = strings.ReplaceAll(msg, a.Token, "REDACTED")
	}
	return msg
}

// ---------------------------------------------------------------------------
// data extraction helpers
// ---------------------------------------------------------------------------

// Field returns data[key] as a generic value.
func Field(data map[string]interface{}, key string) interface{} {
	if data == nil {
		return nil
	}
	return data[key]
}

// ObjectField returns data[key] as a map (a single GraphQL object like
// create_board / create_item), or nil.
func ObjectField(data map[string]interface{}, key string) map[string]interface{} {
	if v, ok := Field(data, key).(map[string]interface{}); ok {
		return v
	}
	return nil
}

// ArrayField returns data[key] as a slice, or an empty slice.
func ArrayField(data map[string]interface{}, key string) []interface{} {
	if v, ok := Field(data, key).([]interface{}); ok {
		return v
	}
	return []interface{}{}
}

// FirstBoard returns data.boards[0] as a map (many queries nest under a single
// board), or nil when the board list is empty.
func FirstBoard(data map[string]interface{}) map[string]interface{} {
	boards := ArrayField(data, "boards")
	if len(boards) == 0 {
		return nil
	}
	b, _ := boards[0].(map[string]interface{})
	return b
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

// ValidateJSON returns the input as a JSON-encoded STRING for Monday's JSON!
// arguments. Empty → "". It reads the connection's raw Value directly (NOT via
// String(), which stringifies a parsed map as a Go literal like "map[label:Done]"
// rather than JSON): a string value is validated and passed through; a
// map/slice value (the field wired from an upstream object) is re-marshalled to
// a JSON string. Mirrors the object-aware JSON handling in the sibling nodes.
func ValidateJSON(name string, inputs []*core.Connection) (string, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return "", nil
	}
	switch v := conn.Value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return "", nil
		}
		var probe interface{}
		if err := json.Unmarshal([]byte(v), &probe); err != nil {
			return "", fmt.Errorf("%s must be valid JSON", name)
		}
		return v, nil
	default:
		// Already a parsed object/array (e.g. wired from an upstream output) —
		// re-encode to a JSON string for Monday.
		b, err := json.Marshal(v)
		if err != nil {
			return "", fmt.Errorf("%s must be valid JSON", name)
		}
		return string(b), nil
	}
}

// ClampLimit bounds a requested page size, falling back to DefaultPageLimit.
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

// ResourceResult shapes a single-resource GraphQL object (create_board,
// create_item, an items[0], …) into the standard action output. The id output
// reads the resource's "id".
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          Stringify(obj["id"]),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// SuccessResult shapes an operation whose id the caller already knows.
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

// ListResult shapes a collection into the standard list output.
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

// Stringify renders a Monday id/value as a clean string (Monday ids are strings,
// but numbers are handled defensively).
func Stringify(v interface{}) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// ItemFields is the shared GraphQL selection for an item — kept in one place so
// every item query returns the same shape.
const ItemFields = `{
	id
	name
	state
	created_at
	updated_at
	column_values {
		id
		text
		value
		type
	}
}`

// BoardFields is the shared selection for a board.
const BoardFields = `id name description state board_kind items_count updated_at`

// CursorItemsAll walks Monday's cursor-based items pagination. `first` is the
// first page's {cursor, items}; when returnAll is true it follows
// next_items_page(cursor) until the cursor is empty (or the page cap). Returns
// the accumulated items.
func CursorItemsAll(a Auth, first map[string]interface{}, returnAll bool) ([]interface{}, error) {
	items := []interface{}{}
	if first == nil {
		return items, nil
	}
	items = append(items, ArrayField(first, "items")...)
	cursor := Stringify(first["cursor"])
	if !returnAll {
		return items, nil
	}
	pages := 0
	for cursor != "" && pages < MaxAllPages {
		data, err := GraphQL(a, `query ($cursor: String!) {
			next_items_page (limit: 100, cursor: $cursor) {
				cursor
				items `+ItemFields+`
			}
		}`, map[string]interface{}{"cursor": cursor})
		if err != nil {
			return nil, err
		}
		page := ObjectField(data, "next_items_page")
		if page == nil {
			break
		}
		items = append(items, ArrayField(page, "items")...)
		cursor = Stringify(page["cursor"])
		pages++
	}
	return items, nil
}

// MaxAllPages bounds a "return all" pagination loop.
const MaxAllPages = 100
