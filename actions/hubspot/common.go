// Package hubspot_common holds the shared HTTP client, auth helpers, and
// generic CRM-object CRUD used by every hubspot/* action.
//
// HubSpot's CRM v3 API is uniform across object types — contacts,
// companies, deals, and tickets all share identical create/read/update/
// delete/list/search shapes under /crm/v3/objects/{objectType}. That
// regularity lets the object CRUD live here once (CreateObject, GetObject,
// UpdateObject, ArchiveObject, ListObjects, SearchObjects), so each action
// package stays thin: read its inputs, call one helper, map the result.
//
// Auth is a HubSpot Private App token carried as a Bearer credential. It is
// modelled as a ConnectionTypeSecret so users paste the long-lived token
// into an environment secret. Swapping to platform-managed OAuth later is a
// one-input change (Secret -> Credential) in AuthInputs and the action
// Inputs literals; GetAPIKey and everything below are unaffected.
package hubspot_common

import (
	"bytes"
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

const (
	// BaseURL is the HubSpot API root. All v3 CRM, v4 association, and
	// legacy v1 list endpoints hang off this host.
	BaseURL = "https://api.hubapi.com"

	// maxResponseBody caps the response body to prevent memory exhaustion.
	maxResponseBody = 1 << 20 // 1 MB

	// requestTimeout is the HTTP client timeout for HubSpot API calls.
	requestTimeout = 30 * time.Second
)

// httpClient is shared across every HubSpot action so TCP connections to
// api.hubapi.com are pooled and reused rather than re-dialled on each call
// (a whole flow run can fire many HubSpot actions). Matches the connection-
// reuse pattern used by the Databricks and OpenAI integrations.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AuthInputs is the shared credential input. Action packages declare their
// own literal Inputs arrays (the manifest generator parses those from the
// AST), but this documents the canonical shape they reuse.
var AuthInputs = []core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "HubSpot Private App Token",
		Placeholder: "pat-...",
		Required:    true,
	},
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ExecuteAPI performs a REST call to the HubSpot API.
// method: GET, POST, PATCH, PUT, DELETE
// path:   absolute path including any query string (e.g. "/crm/v3/objects/contacts/123?properties=email")
// body:   optional payload — marshalled to JSON for POST/PATCH/PUT, ignored for GET/DELETE
func ExecuteAPI(apiKey, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := BaseURL + path

	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPatch || method == http.MethodPut) {
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

	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HubSpot API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Headers:    resp.Header,
	}, nil
}

// CheckResponse verifies the status code is in the 2xx range, decoding
// HubSpot's structured error envelope ({status, message, category}) when
// present so the surfaced error is the human-readable message.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var errResp struct {
		Status   string `json:"status"`
		Message  string `json:"message"`
		Category string `json:"category"`
	}
	if err := json.Unmarshal(resp.Body, &errResp); err == nil && errResp.Message != "" {
		if errResp.Category != "" {
			return fmt.Errorf("HubSpot API error (%d/%s): %s", resp.StatusCode, errResp.Category, errResp.Message)
		}
		return fmt.Errorf("HubSpot API error (%d): %s", resp.StatusCode, errResp.Message)
	}

	return fmt.Errorf("HubSpot API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// decode unmarshals a successful response body into a generic map.
func decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse HubSpot response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// GetAPIKey extracts and validates the private app token from action inputs.
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

// RequiredString extracts a required string input, returning an error if absent.
func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// OptionalInt extracts an integer input. The bool is false when the input is
// absent or empty, so callers can distinguish "unset" from "set to 0".
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Number() == nil {
		return 0, false
	}
	return int(*conn.Number()), true
}

// BuildProperties assembles a HubSpot properties map from two sources, in
// order: the named explicit string inputs (only the non-empty ones), then
// the "additional_properties" key-value array, which can both add new keys
// and override the explicit ones. This gives each action a few first-class
// fields (email, dealname, ...) plus an escape hatch for any other CRM
// property — including custom ones — without a fixed schema.
func BuildProperties(inputs []*core.Connection, fields ...string) map[string]string {
	props := map[string]string{}
	for _, f := range fields {
		if v := OptionalString(f, inputs); v != "" {
			props[f] = v
		}
	}
	if conn := core.FindConnection("additional_properties", inputs); conn != nil {
		for _, pair := range conn.KeyValuePairs() {
			if pair.Key != "" {
				props[pair.Key] = pair.Value
			}
		}
	}
	return props
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

// ErrorResult returns the standard error output map for a graceful action
// failure (returned with a nil error so the engine records it as data).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ---------------------------------------------------------------------------
// Generic CRM object CRUD (objectType ∈ contacts, companies, deals, tickets)
// ---------------------------------------------------------------------------

// CreateObject creates a CRM record of the given object type. associations is
// optional and passed through verbatim as the v3 "associations" array.
func CreateObject(apiKey, objectType string, properties map[string]string, associations []interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{"properties": properties}
	if len(associations) > 0 {
		payload["associations"] = associations
	}
	resp, err := ExecuteAPI(apiKey, http.MethodPost, "/crm/v3/objects/"+objectType, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// GetObject fetches a single record by ID. properties and associations are
// optional projections requested via query string.
func GetObject(apiKey, objectType, id string, properties, associations []string) (map[string]interface{}, error) {
	q := url.Values{}
	if len(properties) > 0 {
		q.Set("properties", strings.Join(properties, ","))
	}
	if len(associations) > 0 {
		q.Set("associations", strings.Join(associations, ","))
	}
	path := "/crm/v3/objects/" + objectType + "/" + url.PathEscape(id)
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	resp, err := ExecuteAPI(apiKey, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// UpdateObject patches the properties of an existing record.
func UpdateObject(apiKey, objectType, id string, properties map[string]string) (map[string]interface{}, error) {
	payload := map[string]interface{}{"properties": properties}
	resp, err := ExecuteAPI(apiKey, http.MethodPatch, "/crm/v3/objects/"+objectType+"/"+url.PathEscape(id), payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// ArchiveObject soft-deletes (archives) a record. HubSpot returns 204 with an
// empty body on success.
func ArchiveObject(apiKey, objectType, id string) error {
	resp, err := ExecuteAPI(apiKey, http.MethodDelete, "/crm/v3/objects/"+objectType+"/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ListObjects returns a page of records. after is the opaque pagination
// cursor from a previous page's paging.next.after (empty for the first page).
func ListObjects(apiKey, objectType string, limit int, after string, properties []string) (map[string]interface{}, error) {
	q := url.Values{}
	if limit > 0 {
		q.Set("limit", strconv.Itoa(limit))
	}
	if after != "" {
		q.Set("after", after)
	}
	if len(properties) > 0 {
		q.Set("properties", strings.Join(properties, ","))
	}
	path := "/crm/v3/objects/" + objectType
	if encoded := q.Encode(); encoded != "" {
		path += "?" + encoded
	}
	resp, err := ExecuteAPI(apiKey, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// BuildSearchBody assembles a v3 CRM search request from the standard search
// inputs shared by every object's search action:
//
//   - query: free-text search across default searchable properties
//   - filter_property / filter_operator / filter_value: one simple filter
//     (operator defaults to EQ) — convenient for the common case
//   - filter_groups: a raw filterGroups array (object input) for advanced
//     queries; when present it takes precedence over the simple filter
//   - properties: comma-separated list of properties to return
//   - limit / after: pagination
//
// HubSpot's EQ/NEQ/GT/LT/... operators are passed through verbatim.
func BuildSearchBody(inputs []*core.Connection) map[string]interface{} {
	body := map[string]interface{}{}

	if q := OptionalString("query", inputs); q != "" {
		body["query"] = q
	}
	if props := CSVToList(OptionalString("properties", inputs)); len(props) > 0 {
		body["properties"] = props
	}
	if limit, ok := OptionalInt("limit", inputs); ok && limit > 0 {
		body["limit"] = limit
	}
	if after := OptionalString("after", inputs); after != "" {
		body["after"] = after
	}

	// Advanced: a raw filterGroups array supplied as an object input wins
	// over the simple single-filter convenience fields.
	if conn := core.FindConnection("filter_groups", inputs); conn != nil && conn.Value != nil {
		if groups, ok := conn.Value.([]interface{}); ok && len(groups) > 0 {
			body["filterGroups"] = groups
			return body
		}
	}

	if prop := OptionalString("filter_property", inputs); prop != "" {
		op := OptionalString("filter_operator", inputs)
		if op == "" {
			op = "EQ"
		}
		filter := map[string]interface{}{"propertyName": prop, "operator": op}
		// HAS_PROPERTY / NOT_HAS_PROPERTY are existence checks that take no
		// value — HubSpot rejects the request if one is supplied, so never
		// attach filter_value for them even if the field was left populated.
		if op != "HAS_PROPERTY" && op != "NOT_HAS_PROPERTY" {
			if v := OptionalString("filter_value", inputs); v != "" {
				filter["value"] = v
			}
		}
		body["filterGroups"] = []interface{}{
			map[string]interface{}{"filters": []interface{}{filter}},
		}
	}

	return body
}

// SearchObjects runs a CRM search. body is the raw v3 search request
// (filterGroups, sorts, query, properties, limit, after).
func SearchObjects(apiKey, objectType string, body map[string]interface{}) (map[string]interface{}, error) {
	resp, err := ExecuteAPI(apiKey, http.MethodPost, "/crm/v3/objects/"+objectType+"/search", body)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// ---------------------------------------------------------------------------
// Associations (v4) and static lists (legacy v1)
// ---------------------------------------------------------------------------

// AssociateDefault creates the default (unlabelled) association between two
// CRM records using the v4 association API. HubSpot infers the association
// type from the object pair, so no association type ID is required.
func AssociateDefault(apiKey, fromType, fromID, toType, toID string) (map[string]interface{}, error) {
	path := fmt.Sprintf("/crm/v4/objects/%s/%s/associations/default/%s/%s",
		fromType, url.PathEscape(fromID), toType, url.PathEscape(toID))
	resp, err := ExecuteAPI(apiKey, http.MethodPut, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// Disassociate removes all associations between two CRM records.
func Disassociate(apiKey, fromType, fromID, toType, toID string) error {
	path := fmt.Sprintf("/crm/v4/objects/%s/%s/associations/%s/%s",
		fromType, url.PathEscape(fromID), toType, url.PathEscape(toID))
	resp, err := ExecuteAPI(apiKey, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ListMembership adds or removes contacts on a static contact list via the
// legacy v1 list API. action is "add" or "remove". vids are numeric contact
// IDs; emails (add only) resolve to contacts by email address.
func ListMembership(apiKey, listID, action string, vids []interface{}, emails []string) (map[string]interface{}, error) {
	body := map[string]interface{}{}
	if len(vids) > 0 {
		body["vids"] = vids
	}
	if len(emails) > 0 {
		body["emails"] = emails
	}
	path := fmt.Sprintf("/contacts/v1/lists/%s/%s", url.PathEscape(listID), action)
	resp, err := ExecuteAPI(apiKey, http.MethodPost, path, body)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// ToInterfaceList widens a []string to []interface{} for JSON payloads.
func ToInterfaceList(items []string) []interface{} {
	out := make([]interface{}, len(items))
	for i, v := range items {
		out[i] = v
	}
	return out
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// summaryWithData appends a JSON encoding of data to a human summary so the
// AI tool-result fallback chain (tool_result -> result -> ...) delivers BOTH
// the readable summary AND the actual record data. The chain uses tool_result
// verbatim when non-empty and never falls through, so a bare summary would
// otherwise starve the AI of the data that lives in the separate output keys.
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

// ObjectResult shapes a single-record API response (create/get/update) into
// the standard action output map: id, properties, the full result object,
// plus a human summary and success flags.
func ObjectResult(obj map[string]interface{}, summary string) map[string]interface{} {
	id, _ := obj["id"].(string)
	return map[string]interface{}{
		"id":          id,
		"properties":  obj["properties"],
		"result":      obj,
		"tool_result": summaryWithData(summary, obj),
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a list/search API response into the standard collection
// output: results array, count, the next-page cursor, summary and flags.
func ListResult(resp map[string]interface{}, summary string) map[string]interface{} {
	results, _ := resp["results"].([]interface{})
	after := ""
	if paging, ok := resp["paging"].(map[string]interface{}); ok {
		if next, ok := paging["next"].(map[string]interface{}); ok {
			after, _ = next["after"].(string)
		}
	}
	return map[string]interface{}{
		"results":     results,
		"count":       len(results),
		"after":       after,
		"result":      resp,
		"tool_result": summaryWithData(summary, results),
		"success":     true,
		"error":       "",
	}
}
