// Package airtable_common holds the shared HTTP client, auth helpers, and the
// record/base helpers used by every airtable/* action.
//
// Airtable's REST API is uniform: record CRUD hangs off
// /v0/{baseId}/{tableIdOrName} and base metadata off /v0/meta/bases. That
// regularity lets the transport, CRUD, and result-shaping helpers live here
// once, so each action package stays thin: read its inputs, call one helper,
// shape the result.
//
// Auth is an Airtable Personal Access Token (PAT) carried as a Bearer
// credential. It is modelled as a ConnectionTypeSecret so users paste the
// long-lived token into an environment secret. Airtable disabled the legacy
// user API key in Feb 2024, so PAT (or OAuth) is the only working method.
// Swapping to platform-managed OAuth later is a one-input change (Secret ->
// Credential) in AuthInputs and the action Inputs literals; GetAccessToken and
// everything below are unaffected.
package airtable_common

import (
	"bytes"
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
	// BaseURL is the Airtable API root, including the /v0 version segment.
	// Record endpoints hang off /{baseId}/{table}; metadata off /meta/bases.
	BaseURL = "https://api.airtable.com/v0"

	// maxResponseBody caps a single response body to prevent memory
	// exhaustion. A list page returns up to 100 records with rich text, so
	// this is more generous than the 1 MB used by lighter integrations.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for Airtable API calls.
	requestTimeout = 30 * time.Second

	// MaxAllPages bounds the "Return All" pagination loop so a misconfigured
	// list can't fetch unboundedly (100 pages * 100 records = 10,000 records).
	// When the cap is hit the remaining offset is returned so callers can
	// continue explicitly.
	MaxAllPages = 100
)

// httpClient is shared across every Airtable action so TCP connections to
// api.airtable.com are pooled and reused rather than re-dialled on each call
// (a flow run can fire many Airtable actions, and list/return-all loops over
// many pages). Matches the connection-reuse pattern used by the HubSpot,
// Databricks, and OpenAI integrations.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AuthInputs is the shared credential input. Action packages declare their own
// literal Inputs arrays (the manifest generator parses those from the AST),
// but this documents the canonical shape they reuse.
var AuthInputs = []core.Connection{
	{
		Name:        "access_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Airtable Personal Access Token",
		Placeholder: "pat...",
		Required:    true,
	},
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ExecuteAPI performs a REST call to the Airtable API.
// method: GET, POST, PATCH, PUT, DELETE
// path:   absolute path below /v0, including any query string
//
//	(e.g. "/appXXX/Table%201?maxRecords=10" or "/meta/bases")
//
// body: optional payload — marshalled to JSON for POST/PATCH/PUT, ignored for
// GET/DELETE.
func ExecuteAPI(token, method, path string, body interface{}) (*APIResponse, error) {
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

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Airtable API request failed: %w", err)
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
// Airtable's error envelope for a human-readable message. Airtable returns
// either {"error":{"type","message"}} or, for some 4xx responses, a bare
// {"error":"NOT_FOUND"} string.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var structured struct {
		Error struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &structured); err == nil && structured.Error.Message != "" {
		if structured.Error.Type != "" {
			return fmt.Errorf("Airtable API error (%d/%s): %s", resp.StatusCode, structured.Error.Type, structured.Error.Message)
		}
		return fmt.Errorf("Airtable API error (%d): %s", resp.StatusCode, structured.Error.Message)
	}

	var stringErr struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &stringErr); err == nil && stringErr.Error != "" {
		return fmt.Errorf("Airtable API error (%d): %s", resp.StatusCode, stringErr.Error)
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		return fmt.Errorf("Airtable API error (429): rate limit exceeded — Airtable allows 5 requests/second/base and enforces a 30-second cooldown after a burst")
	}

	return fmt.Errorf("Airtable API error (%d): %s", resp.StatusCode, string(resp.Body))
}

// decode unmarshals a successful response body into a generic map.
func decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Airtable response: %w", err)
	}
	return out, nil
}

// recordPath builds the collection path for a table, escaping the base ID and
// table (the table may be a human name containing spaces, e.g. "Table 1").
func recordPath(baseID, table string) string {
	return "/" + url.PathEscape(baseID) + "/" + url.PathEscape(table)
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// GetAccessToken extracts and validates the PAT from action inputs.
func GetAccessToken(inputs []*core.Connection) (string, error) {
	conn := core.FindConnection("access_token", inputs)
	if conn == nil || conn.String() == nil || *conn.String() == "" {
		return "", fmt.Errorf("access_token is required")
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

// BuildFields assembles the Airtable "fields" object from two inputs, in
// order: an advanced JSON object ("fields") carrying full-fidelity typed
// values (arrays for multi-selects / linked records / attachments, numbers,
// booleans), then a simple key/value list ("fields_kv") whose string entries
// are overlaid on top. The key/value rows win on conflict, so a user can bulk
// set via JSON and tweak individual fields with a row. Returns an empty map
// when neither is supplied.
func BuildFields(inputs []*core.Connection) (map[string]interface{}, error) {
	fields := map[string]interface{}{}

	if conn := core.FindConnection("fields", inputs); conn != nil && conn.Value != nil {
		switch v := conn.Value.(type) {
		case map[string]interface{}:
			// Engine already parsed the object input into a map.
			for k, val := range v {
				fields[k] = val
			}
		case string:
			if err := mergeJSONObject(v, fields); err != nil {
				return nil, err
			}
		default:
			// Any other encoding — round-trip through String() to JSON text.
			if s := conn.String(); s != nil {
				if err := mergeJSONObject(*s, fields); err != nil {
					return nil, err
				}
			}
		}
	}

	if conn := core.FindConnection("fields_kv", inputs); conn != nil {
		for _, pair := range conn.KeyValuePairs() {
			if pair.Key != "" {
				fields[pair.Key] = pair.Value
			}
		}
	}

	return fields, nil
}

// mergeJSONObject parses raw as a JSON object and merges it into dst. Empty,
// "{}" and "null" inputs are treated as no-ops.
func mergeJSONObject(raw string, dst map[string]interface{}) error {
	raw = strings.TrimSpace(raw)
	if raw == "" || raw == "{}" || raw == "null" {
		return nil
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return fmt.Errorf("fields must be a JSON object: %w", err)
	}
	for k, val := range parsed {
		dst[k] = val
	}
	return nil
}

// BuildListQuery assembles the query string for a record list request from the
// standard search inputs (filterByFormula, view, field projection, sort). The
// caller adds pagination params (pageSize, maxRecords, offset).
func BuildListQuery(inputs []*core.Connection) (url.Values, error) {
	q := url.Values{}

	if f := OptionalString("filter_by_formula", inputs); f != "" {
		q.Set("filterByFormula", f)
	}
	if v := OptionalString("view", inputs); v != "" {
		q.Set("view", v)
	}
	for _, name := range CSVToList(OptionalString("return_fields", inputs)) {
		q.Add("fields[]", name)
	}

	// Sorting: an advanced JSON array [{"field","direction"}] wins; otherwise
	// the simple sort_field / sort_direction pair. The output index is tracked
	// separately from the input index so skipped (blank-field) entries don't
	// leave gaps in the sort[i] keys.
	specs, err := sortSpecs(inputs)
	if err != nil {
		return nil, err
	}
	if len(specs) > 0 {
		idx := 0
		for _, s := range specs {
			field, _ := s["field"].(string)
			if field == "" {
				continue
			}
			q.Set(fmt.Sprintf("sort[%d][field]", idx), field)
			if dir, ok := s["direction"].(string); ok && dir != "" {
				q.Set(fmt.Sprintf("sort[%d][direction]", idx), dir)
			}
			idx++
		}
	} else if sf := OptionalString("sort_field", inputs); sf != "" {
		q.Set("sort[0][field]", sf)
		dir := OptionalString("sort_direction", inputs)
		if dir == "" {
			dir = "asc"
		}
		q.Set("sort[0][direction]", dir)
	}

	return q, nil
}

// sortSpecs extracts the advanced sort specification from the "sort" object
// input. As with the "fields" object handled by BuildFields, the engine may
// deliver this as a JSON string OR an already-parsed []interface{} (e.g. when
// the input is wired to an upstream array output). Reading it via String()
// would Go-format a parsed slice ("[map[...]]") and then fail to re-parse, so
// switch on the concrete value type instead. Returns nil for an absent/empty
// input and an error for malformed JSON or a non-array value.
func sortSpecs(inputs []*core.Connection) ([]map[string]interface{}, error) {
	conn := core.FindConnection("sort", inputs)
	if conn == nil || conn.Value == nil {
		return nil, nil
	}

	toSpecs := func(arr []interface{}) []map[string]interface{} {
		out := make([]map[string]interface{}, 0, len(arr))
		for _, item := range arr {
			if m, ok := item.(map[string]interface{}); ok {
				out = append(out, m)
			}
		}
		return out
	}

	parse := func(raw string) ([]map[string]interface{}, error) {
		raw = strings.TrimSpace(raw)
		if raw == "" || raw == "[]" || raw == "null" {
			return nil, nil
		}
		var arr []interface{}
		if err := json.Unmarshal([]byte(raw), &arr); err != nil {
			return nil, fmt.Errorf(`sort must be a JSON array like [{"field":"Name","direction":"asc"}]: %w`, err)
		}
		return toSpecs(arr), nil
	}

	switch v := conn.Value.(type) {
	case []interface{}:
		return toSpecs(v), nil
	case string:
		return parse(v)
	default:
		return nil, fmt.Errorf(`sort must be a JSON array like [{"field":"Name","direction":"asc"}]`)
	}
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
// Record CRUD
// ---------------------------------------------------------------------------

// CreateRecord creates a single record. When typecast is true Airtable coerces
// string values to the field's type (and creates missing select options).
func CreateRecord(token, baseID, table string, fields map[string]interface{}, typecast bool) (map[string]interface{}, error) {
	payload := map[string]interface{}{"fields": fields}
	if typecast {
		payload["typecast"] = true
	}
	resp, err := ExecuteAPI(token, http.MethodPost, recordPath(baseID, table), payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// GetRecord fetches a single record by ID.
func GetRecord(token, baseID, table, recordID string) (map[string]interface{}, error) {
	path := recordPath(baseID, table) + "/" + url.PathEscape(recordID)
	resp, err := ExecuteAPI(token, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// UpdateRecord patches the fields of a single record by ID. typecast behaves as
// in CreateRecord.
func UpdateRecord(token, baseID, table, recordID string, fields map[string]interface{}, typecast bool) (map[string]interface{}, error) {
	payload := map[string]interface{}{"fields": fields}
	if typecast {
		payload["typecast"] = true
	}
	path := recordPath(baseID, table) + "/" + url.PathEscape(recordID)
	resp, err := ExecuteAPI(token, http.MethodPatch, path, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// DeleteRecord deletes a single record by ID. Airtable returns
// {"id":"...","deleted":true}.
func DeleteRecord(token, baseID, table, recordID string) (map[string]interface{}, error) {
	path := recordPath(baseID, table) + "/" + url.PathEscape(recordID)
	resp, err := ExecuteAPI(token, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// UpsertRecord creates or updates a single record using Airtable's native
// upsert: it PATCHes the table collection with performUpsert.fieldsToMergeOn.
// Airtable updates the record whose merge fields all equal the given values,
// or creates a new record when none match. The response carries records plus
// createdRecords / updatedRecords ID lists. Each merge field must be present in
// fields.
func UpsertRecord(token, baseID, table string, fields map[string]interface{}, mergeOn []string, typecast bool) (map[string]interface{}, error) {
	payload := map[string]interface{}{
		"performUpsert": map[string]interface{}{"fieldsToMergeOn": mergeOn},
		"records":       []interface{}{map[string]interface{}{"fields": fields}},
	}
	if typecast {
		payload["typecast"] = true
	}
	resp, err := ExecuteAPI(token, http.MethodPatch, recordPath(baseID, table), payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// ListRecordsPage fetches one page of records. q carries the query string
// (filterByFormula, sort, view, fields, pageSize, maxRecords, offset). It
// returns the page's records, the next-page offset ("" when exhausted), and
// the raw response.
func ListRecordsPage(token, baseID, table string, q url.Values) (records []interface{}, offset string, raw map[string]interface{}, err error) {
	path := recordPath(baseID, table)
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	resp, err := ExecuteAPI(token, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, "", nil, err
	}
	raw, err = decode(resp)
	if err != nil {
		return nil, "", nil, err
	}
	records, _ = raw["records"].([]interface{})
	offset, _ = raw["offset"].(string)
	return records, offset, raw, nil
}

// ---------------------------------------------------------------------------
// Base metadata
// ---------------------------------------------------------------------------

// ListBasesPage fetches one page of the bases the token can access
// (GET /meta/bases). offset is the cursor from a previous page ("" for the
// first). Requires the schema.bases:read scope.
func ListBasesPage(token, offset string) (bases []interface{}, next string, raw map[string]interface{}, err error) {
	path := "/meta/bases"
	if offset != "" {
		q := url.Values{}
		q.Set("offset", offset)
		path += "?" + q.Encode()
	}
	resp, err := ExecuteAPI(token, http.MethodGet, path, nil)
	if err != nil {
		return nil, "", nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, "", nil, err
	}
	raw, err = decode(resp)
	if err != nil {
		return nil, "", nil, err
	}
	bases, _ = raw["bases"].([]interface{})
	next, _ = raw["offset"].(string)
	return bases, next, raw, nil
}

// GetBaseSchema fetches the tables (and their fields/views) of a base
// (GET /meta/bases/{baseId}/tables). Requires the schema.bases:read scope.
func GetBaseSchema(token, baseID string) (map[string]interface{}, error) {
	path := "/meta/bases/" + url.PathEscape(baseID) + "/tables"
	resp, err := ExecuteAPI(token, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// summaryWithData embeds the JSON payload in tool_result so an AI caller
// actually receives the data. The engine's tool-result fallback chain uses
// tool_result verbatim when non-empty and never falls through to the data
// outputs, so a bare summary meant gets/lists/reports reached the model with
// none of the actual data. The engine applies token-budget-aware truncation
// downstream, so large payloads degrade gracefully.
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

// RecordResult shapes a single-record response (create/get/update) into the
// standard action output: id, fields, the full record, plus summary and flags.
func RecordResult(rec map[string]interface{}, summary string) map[string]interface{} {
	id, _ := rec["id"].(string)
	return map[string]interface{}{
		"id":          id,
		"fields":      rec["fields"],
		"record":      rec,
		"tool_result": summaryWithData(summary, rec),
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard output: records,
// count, the next-page offset, the raw response, summary and flags.
func ListResult(records []interface{}, offset string, raw map[string]interface{}, summary string) map[string]interface{} {
	return map[string]interface{}{
		"records":     records,
		"count":       len(records),
		"offset":      offset,
		"result":      raw,
		"tool_result": summaryWithData(summary, records),
		"success":     true,
		"error":       "",
	}
}
