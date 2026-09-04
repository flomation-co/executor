// Package freshsales_common is the Freshsales half of the Freshworks
// integration: the API path prefix, the input helpers every action shares, and
// the result shapes.
//
// The transport — bundle validation, auth header, the HTTP client — lives in
// freshworks_common, so a future Freshdesk or Freshservice integration inherits
// the security-critical part rather than copying it.
//
// No Execute function, so the manifest generator skips this package.
package freshsales_common

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	freshworks_common "flomation.app/automate/executor/actions/crm/freshworks"
)

// APIPrefix is the Freshsales path under a bundle's origin.
const APIPrefix = "/crm/sales/api"

// AuthInputs are the two inputs every Freshsales action carries. Declared here
// as a helper for readability, but each action still inlines its own Inputs
// literal — the manifest generator resolves only literal composite literals, so
// a shared slice would produce an empty inputs list.
const (
	InputAPIKey  = "api_key"
	InputAccount = "account"
)

// Client validates the operator's bundle and returns a per-call client.
func Client(inputs []*core.Connection) (*freshworks_common.Client, error) {
	apiKey := strings.TrimSpace(OptionalString(InputAPIKey, inputs))
	if apiKey == "" {
		return nil, fmt.Errorf("an API key is required — Profile Settings ▸ API Settings in Freshsales")
	}
	if strings.Contains(apiKey, "${") {
		return nil, fmt.Errorf("the API key still contains an unresolved ${...} reference — check the secret exists in this environment")
	}

	origin, err := freshworks_common.ValidateBundle(OptionalString(InputAccount, inputs))
	if err != nil {
		return nil, err
	}
	return freshworks_common.NewClient(origin, apiKey, APIPrefix), nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return strings.TrimSpace(*c.String())
}

func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
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

// SetString copies a non-empty input into the request body.
func SetString(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalString(name, inputs); v != "" {
		body[field] = v
	}
}

// SetInt copies a set integer input into the request body.
func SetInt(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalInt(name, inputs); v != nil {
		body[field] = *v
	}
}

// SetBool copies a set boolean input into the request body.
func SetBool(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalBool(name, inputs); v != nil {
		body[field] = *v
	}
}

// SetNumber copies a decimal input (deal value, product price) into the body.
func SetNumber(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return
	}
	if f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
		body[field] = f
	}
}

// SetIDList turns a comma-separated list of ids into a JSON array.
func SetIDList(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	ids := IDList(name, inputs)
	if len(ids) > 0 {
		body[field] = ids
	}
}

// IDList parses a comma-separated id input. Non-numeric entries are kept as
// strings, because some Freshsales ids arrive as strings in practice.
func IDList(name string, inputs []*core.Connection) []interface{} {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}
	out := []interface{}{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if n, err := strconv.ParseInt(part, 10, 64); err == nil {
			out = append(out, n)
			continue
		}
		out = append(out, part)
	}
	return out
}

// ParseJSONObject reads an optional JSON-object input (the `fields` escape
// hatch every create/update action carries).
func ParseJSONObject(name string, inputs []*core.Connection) (map[string]interface{}, error) {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("%s must be a JSON object: %w", name, err)
	}
	return out, nil
}

// ParseJSONArray reads an optional JSON-array input (bulk payloads).
func ParseJSONArray(name string, inputs []*core.Connection) ([]interface{}, error) {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil, nil
	}
	var out []interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return nil, fmt.Errorf("%s must be a JSON array: %w", name, err)
	}
	return out, nil
}

// MergeFields folds the `fields` escape hatch over the curated inputs, so an
// author can reach a field we did not model without waiting for a new action.
func MergeFields(body map[string]interface{}, extra map[string]interface{}) {
	for k, v := range extra {
		body[k] = v
	}
}

// Query builds the common paging/sorting query string.
func Query(inputs []*core.Connection, extra map[string]string) url.Values {
	q := url.Values{}
	if page := OptionalInt("page", inputs); page != nil && *page > 0 {
		q.Set("page", strconv.FormatInt(*page, 10))
	}
	if per := OptionalInt("per_page", inputs); per != nil && *per > 0 {
		q.Set("per_page", strconv.FormatInt(*per, 10))
	}
	if v := OptionalString("sort", inputs); v != "" {
		q.Set("sort", v)
	}
	if v := OptionalString("sort_type", inputs); v != "" {
		q.Set("sort_type", v)
	}
	if v := OptionalString("include", inputs); v != "" {
		q.Set("include", v)
	}
	for k, v := range extra {
		if v != "" {
			q.Set(k, v)
		}
	}
	return q
}

// ---------------------------------------------------------------------------
// Response helpers
// ---------------------------------------------------------------------------

// Obj pulls a nested object out of a Freshworks response.
func Obj(resp map[string]interface{}, key string) map[string]interface{} {
	if resp == nil {
		return nil
	}
	if v, ok := resp[key].(map[string]interface{}); ok {
		return v
	}
	return nil
}

// Arr pulls a nested array out of a Freshworks response.
func Arr(resp map[string]interface{}, key string) []interface{} {
	if resp == nil {
		return nil
	}
	if v, ok := resp[key].([]interface{}); ok {
		return v
	}
	return nil
}

// IDOf reads a record's id, whatever numeric shape it arrived in.
func IDOf(obj map[string]interface{}) string {
	if obj == nil {
		return ""
	}
	switch v := obj["id"].(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	}
	return ""
}

// NameOf reads whichever field a record uses as its human label.
func NameOf(obj map[string]interface{}) string {
	for _, key := range []string{"display_name", "name", "title", "subject"} {
		if v, ok := obj[key].(string); ok && v != "" {
			return v
		}
	}
	first, _ := obj["first_name"].(string)
	last, _ := obj["last_name"].(string)
	if joined := strings.TrimSpace(first + " " + last); joined != "" {
		return joined
	}
	if v, ok := obj["email"].(string); ok {
		return v
	}
	return ""
}

// toolResultWithData embeds the payload in the AI-visible result.
//
// The engine uses tool_result verbatim whenever it is non-empty and never falls
// through to `result`, so a bare summary means the model gets the sentence and
// none of the data. See feedback_tool_result_must_embed_data.
func toolResultWithData(summary string, payload interface{}) string {
	if payload == nil {
		return summary
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return summary
	}
	if summary == "" {
		return string(encoded)
	}
	return summary + "\n" + string(encoded)
}

// ObjectResult is the standard single-record success shape.
func ObjectResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"tool_result": toolResultWithData(summary, obj),
		"id":          IDOf(obj),
		"result":      obj,
		"success":     true,
		"error":       "",
	}
}

// ListResult is the standard collection success shape.
func ListResult(items []interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"tool_result": toolResultWithData(summary, items),
		"results":     items,
		"count":       int64(len(items)),
		"success":     true,
		"error":       "",
	}
}

// PlainResult is for calls whose useful answer is the whole response (bulk
// jobs, selectors, deletes that echo state).
func PlainResult(payload map[string]interface{}, summary string) map[string]interface{} {
	if payload == nil {
		payload = map[string]interface{}{}
	}
	return map[string]interface{}{
		"tool_result": toolResultWithData(summary, payload),
		"result":      payload,
		"success":     true,
		"error":       "",
	}
}

// OkResult is for calls with no meaningful body (delete, mark done).
func OkResult(summary string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": summary,
		"result":      map[string]interface{}{},
		"success":     true,
		"error":       "",
	}
}

// ErrorResult is a graceful failure: the node succeeds, success is false, and
// the reason is readable by both a person and an agent.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"result":      map[string]interface{}{},
		"results":     []interface{}{},
		"count":       int64(0),
		"id":          "",
		"success":     false,
		"error":       msg,
	}
}

// TargetablePath maps a record type to its URL segment.
//
// Freshsales names the type "SalesAccount" in request bodies but
// "sales_accounts" in paths, so lowercasing the type is wrong for exactly the
// case people most often get wrong. An unknown type returns "" so the caller
// can refuse rather than build a URL that 404s.
func TargetablePath(targetableType string) string {
	switch strings.ToLower(strings.TrimSpace(targetableType)) {
	case "contact", "contacts":
		return "contacts"
	case "salesaccount", "sales_account", "sales_accounts", "account", "accounts":
		return "sales_accounts"
	case "deal", "deals":
		return "deals"
	}
	return ""
}

// knownSelectors is the closed set of Freshsales configuration endpoints.
//
// Validated rather than interpolated blindly: the value reaches a URL path, and
// an unchecked one would let a crafted input reach a different endpoint on the
// customer's own account.
var knownSelectors = map[string]bool{
	"owners": true, "territories": true, "deal_stages": true, "deal_types": true,
	"deal_reasons": true, "deal_payment_statuses": true, "currencies": true,
	"lead_sources": true, "industry_types": true, "business_types": true,
	"campaigns": true, "contact_statuses": true, "lifecycle_stages": true,
	"sales_activity_types": true, "sales_activity_outcomes": true,
}

// IsKnownSelector reports whether a selector name is one Freshsales publishes.
func IsKnownSelector(name string) bool {
	return knownSelectors[strings.TrimSpace(strings.ToLower(name))]
}
