// Package quickbooks_common holds the shared auth inputs, REST client, input
// helpers and result shapers for every QuickBooks Online action. It has no
// Execute function, so the manifest generator excludes it from the registry.
//
// Auth model: QuickBooks is an OAuth2 managed credential. Each action takes a
// `credential` input resolving to the access token (${credentials.X}) and a
// `company` input resolving to the company/realm id (${credentials.X.realm_id})
// which the editor auto-fills from the chosen credential. QBO addresses every
// resource under /v3/company/<realmId>/, so the realm id goes in the URL (not a
// header as with Xero's tenant). Token refresh is handled server-side.
package quickbooks_common

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

// reqContext returns the flow's cancellation context, or a background context
// when flow is nil (unit tests drive Execute with a nil flow).
func reqContext(flow *core.Flow) context.Context {
	if flow == nil {
		return context.Background()
	}
	return flow.GoContext()
}

// ProductionBaseURL / SandboxBaseURL are the QBO API roots. Vars so tests can
// point them at an httptest server.
var (
	ProductionBaseURL = "https://quickbooks.api.intuit.com"
	SandboxBaseURL    = "https://sandbox-quickbooks.api.intuit.com"
)

// minorVersion pins the QBO API minor version for stable field behaviour.
const minorVersion = "70"

const requestTimeout = 30 * time.Second

// AuthInputs is the shared credential + company (+ sandbox) inputs every QBO
// action embeds first. `credential` resolves to the access token; `company`
// resolves to the realm id; `sandbox` routes to the sandbox host for test data.
var AuthInputs = []core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox", Placeholder: "Route to the QuickBooks sandbox company"},
}

// StandardOutputs is the common output contract (tool_result first for AI use).
var StandardOutputs = []core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// Auth carries the resolved connection details for a single call.
type Auth struct {
	Token   string
	Realm   string
	Sandbox bool
}

// GetAuth extracts the resolved access token, realm id and sandbox flag.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	token, err := RequiredString("credential", inputs)
	if err != nil {
		return Auth{}, fmt.Errorf("connect a QuickBooks account: %w", err)
	}
	realm, err := RequiredString("company", inputs)
	if err != nil {
		return Auth{}, fmt.Errorf("missing QuickBooks company (realm id): %w", err)
	}
	if strings.HasPrefix(token, "${") || strings.HasPrefix(realm, "${") {
		return Auth{}, fmt.Errorf("QuickBooks credential did not resolve — connect and authorise a QuickBooks account in this environment")
	}
	sandbox := false
	if b := OptionalBool("sandbox", inputs); b != nil {
		sandbox = *b
	}
	return Auth{Token: token, Realm: realm, Sandbox: sandbox}, nil
}

func (a Auth) baseURL() string {
	if a.Sandbox {
		return SandboxBaseURL
	}
	return ProductionBaseURL
}

// --- REST client ---

// Post creates or updates an entity (QBO uses POST to /<entity> for both; a
// sparse update includes "sparse":true and the Id/SyncToken). Returns the
// decoded response. `entity` may itself carry a query string (e.g.
// "invoice?operation=void" or "invoice/{id}/send?sendTo=…"), in which case the
// minorversion is appended with & rather than ? to keep the URL valid.
func Post(flow *core.Flow, a Auth, entity string, body interface{}) (map[string]interface{}, error) {
	sep := "?"
	if strings.Contains(entity, "?") {
		sep = "&"
	}
	path := fmt.Sprintf("/v3/company/%s/%s%sminorversion=%s", url.PathEscape(a.Realm), entity, sep, minorVersion)
	return do(flow, a, http.MethodPost, path, body)
}

// GetByID reads a single entity by id: GET /<entity>/<id>.
func GetByID(flow *core.Flow, a Auth, entity, id string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/v3/company/%s/%s/%s?minorversion=%s", url.PathEscape(a.Realm), entity, url.PathEscape(id), minorVersion)
	return do(flow, a, http.MethodGet, path, nil)
}

// Query runs a QBO SQL-like query: GET /query?query=<SQL>.
func Query(flow *core.Flow, a Auth, sql string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/v3/company/%s/query?query=%s&minorversion=%s", url.PathEscape(a.Realm), url.QueryEscape(sql), minorVersion)
	return do(flow, a, http.MethodGet, path, nil)
}

// Report fetches a named report: GET /reports/<name>?<params>.
func Report(flow *core.Flow, a Auth, name string, params url.Values) (map[string]interface{}, error) {
	if params == nil {
		params = url.Values{}
	}
	params.Set("minorversion", minorVersion)
	path := fmt.Sprintf("/v3/company/%s/reports/%s?%s", url.PathEscape(a.Realm), url.PathEscape(name), params.Encode())
	return do(flow, a, http.MethodGet, path, nil)
}

func do(flow *core.Flow, a Auth, method, path string, body interface{}) (map[string]interface{}, error) {
	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(reqContext(flow), method, a.baseURL()+path, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := (&http.Client{Timeout: requestTimeout}).Do(req)
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
			return nil, fmt.Errorf("unable to parse QuickBooks response: %w", err)
		}
	}
	return decoded, nil
}

// Entity pulls the named entity object out of a QBO response ({"Customer":{...}}).
func Entity(resp map[string]interface{}, name string) map[string]interface{} {
	obj, _ := resp[name].(map[string]interface{})
	return obj
}

// QueryRows returns the rows of the given entity from a query response
// ({"QueryResponse":{"Customer":[...]}}).
func QueryRows(resp map[string]interface{}, entity string) []map[string]interface{} {
	qr, ok := resp["QueryResponse"].(map[string]interface{})
	if !ok {
		return nil
	}
	arr, ok := qr[entity].([]interface{})
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

// IDOf reads the "Id" field of a QBO entity object.
func IDOf(obj map[string]interface{}) string {
	if obj == nil {
		return ""
	}
	id, _ := obj["Id"].(string)
	return id
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

// SetString assigns a string field into the body map when present.
func SetString(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalString(name, inputs); v != "" {
		body[field] = v
	}
}

// SetNumber parses a money/number input (major-unit decimal, e.g. "100.00") and
// assigns it as a JSON number. QBO uses major units, so no conversion needed.
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

// RefField builds a QBO reference object ({"value":"<id>"}) for a *Ref field.
func RefField(id string) map[string]interface{} {
	return map[string]interface{}{"value": id}
}

// ParseJSONObject reads a text input as a JSON object (advanced/nested fields).
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

// ParseJSONArray reads a text input as a JSON array (e.g. Line items).
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

func ObjectResult(id string, obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"tool_result": summary,
		"id":          id,
		"result":      obj,
		"success":     true,
		"error":       "",
	}
}

func ListResult(items []map[string]interface{}, summary string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": summary,
		"results":     items,
		"count":       len(items),
		"success":     true,
		"error":       "",
	}
}

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": "Error: " + msg,
		"success":     false,
		"error":       msg,
	}
}

// MapError turns an API/transport error into a graceful ErrorResult, extracting
// QuickBooks' Fault message where possible.
func MapError(err error) map[string]interface{} {
	if ae, ok := err.(*apiError); ok {
		return ErrorResult(ae.Error())
	}
	return ErrorResult(err.Error())
}

type apiError struct {
	status int
	body   []byte
}

func (e *apiError) Error() string {
	// QBO error shape: {"Fault":{"Error":[{"Message":"...","Detail":"..."}]}}
	var x struct {
		Fault struct {
			Error []struct {
				Message string `json:"Message"`
				Detail  string `json:"Detail"`
				Code    string `json:"code"`
			} `json:"Error"`
		} `json:"Fault"`
	}
	if json.Unmarshal(e.body, &x) == nil && len(x.Fault.Error) > 0 {
		er := x.Fault.Error[0]
		msg := er.Message
		if er.Detail != "" {
			msg += ": " + er.Detail
		}
		if msg != "" {
			return msg
		}
	}
	body := string(e.body)
	if len(body) > 300 {
		body = body[:300]
	}
	return fmt.Sprintf("QuickBooks API returned %d: %s", e.status, body)
}
