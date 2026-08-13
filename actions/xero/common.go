// Package xero_common holds the shared auth inputs, REST client, input helpers
// and result shapers for every Xero action. It has no Execute function, so the
// manifest generator excludes it from the action registry.
//
// Auth model: Xero is an OAuth2 managed credential (see the api credential
// system). Each action takes a `credential` input that resolves to the current
// access token (${credentials.X}) and a `tenant` input that resolves to the
// organisation's tenant id (${credentials.X.tenant_id}) — the editor auto-fills
// the tenant from the chosen credential. Both reach Execute already substituted
// to plain strings, so this package never touches the credential lifecycle;
// token refresh is handled server-side by the API's refresh poller.
package xero_common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
)

// reqContext returns the flow's cancellation context, or a background context
// when flow is nil (unit tests drive Execute with a nil flow).
func reqContext(flow *core.Flow) context.Context {
	if flow == nil {
		return context.Background()
	}
	return flow.GoContext()
}

// BaseURL is the Xero Accounting API root. A var so tests can point it at an
// httptest server.
var BaseURL = "https://api.xero.com/api.xro/2.0"

const requestTimeout = 30 * time.Second

// AuthInputs is the shared credential + tenant pair every Xero action embeds
// first in its Inputs. `credential` resolves to the access token; `tenant`
// resolves to the Xero organisation's tenant id.
var AuthInputs = []core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
}

// StandardOutputs is the common output contract (tool_result first for AI use).
var StandardOutputs = []core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// --- auth ---

// GetAuth extracts the resolved access token and tenant id. Returns a friendly
// error if either is missing or a ${credentials...} reference failed to resolve
// (which happens when no Xero account is connected in the environment).
func GetAuth(inputs []*core.Connection) (token, tenant string, err error) {
	token, err = RequiredString("credential", inputs)
	if err != nil {
		return "", "", fmt.Errorf("connect a Xero account: %w", err)
	}
	tenant, err = RequiredString("tenant", inputs)
	if err != nil {
		return "", "", fmt.Errorf("missing Xero organisation (tenant): %w", err)
	}
	if strings.HasPrefix(token, "${") || strings.HasPrefix(tenant, "${") {
		return "", "", fmt.Errorf("Xero credential did not resolve — connect and authorise a Xero account in this environment")
	}
	return token, tenant, nil
}

// --- REST client ---

// DoJSON performs an authenticated Xero API request. `path` is appended to
// BaseURL (e.g. "/Contacts" or "/Invoices/<id>"). body is marshalled as JSON
// when non-nil. Returns the decoded response map on 2xx, or a graceful error
// result map (never a node error) on any API failure.
func DoJSON(flow *core.Flow, method, path, token, tenant string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}

	url := BaseURL + path
	req, err := http.NewRequestWithContext(reqContext(flow), method, url, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Xero-Tenant-Id", tenant)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: requestTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiError{status: resp.StatusCode, body: respBody}
	}

	var decoded map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &decoded); err != nil {
			return nil, fmt.Errorf("unable to parse Xero response: %w", err)
		}
	}
	return decoded, nil
}

// FirstElement returns the first object from a Xero collection response. Xero
// wraps returned objects in a top-level plural key (Contacts, Invoices, …) that
// holds an array — this pulls out element [0] for create/get-by-id calls.
func FirstElement(resp map[string]interface{}, collection string) map[string]interface{} {
	arr, ok := resp[collection].([]interface{})
	if !ok || len(arr) == 0 {
		return nil
	}
	obj, _ := arr[0].(map[string]interface{})
	return obj
}

// Elements returns the full slice under a Xero collection key as maps.
func Elements(resp map[string]interface{}, collection string) []map[string]interface{} {
	arr, ok := resp[collection].([]interface{})
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

// --- input helpers ---

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

func OptionalBool(name string, inputs []*core.Connection) *bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	return c.Boolean()
}

// SetString assigns a string field into the body map when the input is present.
func SetString(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalString(name, inputs); v != "" {
		body[field] = v
	}
}

// SetNumber parses a money/number input (major-unit decimal string, e.g.
// "100.00") and assigns it as a JSON number. Xero accepts amounts in major
// units, so no minor-unit conversion is needed. A blank/invalid value is
// skipped.
func SetNumber(body map[string]interface{}, field, name string, inputs []*core.Connection) {
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

// ParseJSONObject reads a text input as a JSON object (for advanced/nested
// fields not surfaced as scalars). Returns nil when blank.
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

// ParseJSONArray reads a text input as a JSON array (e.g. line items). Returns
// nil when blank.
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

// --- result shapers ---

// ObjectResult wraps a single Xero object (already a map). The id is read from
// the object's own id field (the field name varies by resource, so callers pass
// the id explicitly).
func ObjectResult(id string, obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"tool_result": summaryWithData(summary, obj),
		"id":          id,
		"result":      obj,
		"success":     true,
		"error":       "",
	}
}

// ListResult wraps a slice of Xero objects.
func ListResult(items []map[string]interface{}, summary string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": summaryWithData(summary, items),
		"results":     items,
		"count":       len(items),
		"success":     true,
		"error":       "",
	}
}

// summaryWithData embeds the JSON payload in tool_result so an AI caller
// actually receives the data. The engine's tool-result fallback chain uses
// tool_result verbatim whenever it is non-empty and never falls through to the
// `result`/`results` outputs — so a bare summary meant reports, gets and lists
// reached the model as "Fetched … report" with none of the actual figures.
// The engine applies token-budget-aware truncation downstream, so large
// payloads (e.g. Profit and Loss) degrade gracefully rather than being dropped.
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

// ErrorResult is a graceful failure — success=false, not a node error.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"success":     false,
		"error":       msg,
	}
}

// MapError turns an API/transport error into a graceful ErrorResult, extracting
// Xero's human-readable message where possible.
func MapError(err error) map[string]interface{} {
	if ae, ok := err.(*apiError); ok {
		return ErrorResult(ae.Error())
	}
	return ErrorResult(err.Error())
}

// apiError carries a non-2xx Xero response and renders its message.
type apiError struct {
	status int
	body   []byte
}

func (e *apiError) Error() string {
	// Xero validation/error shape: {"Type":"...","Message":"...",
	//   "Elements":[{"ValidationErrors":[{"Message":"..."}]}]}
	var x struct {
		Message  string `json:"Message"`
		Detail   string `json:"detail"`
		Elements []struct {
			ValidationErrors []struct {
				Message string `json:"Message"`
			} `json:"ValidationErrors"`
		} `json:"Elements"`
	}
	if json.Unmarshal(e.body, &x) == nil {
		var parts []string
		if x.Message != "" {
			parts = append(parts, x.Message)
		}
		for _, el := range x.Elements {
			for _, ve := range el.ValidationErrors {
				if ve.Message != "" {
					parts = append(parts, ve.Message)
				}
			}
		}
		if x.Detail != "" {
			parts = append(parts, x.Detail)
		}
		if len(parts) > 0 {
			return strings.Join(parts, "; ")
		}
	}
	body := string(e.body)
	if len(body) > 300 {
		body = body[:300]
	}
	return fmt.Sprintf("Xero API returned %d: %s", e.status, body)
}
