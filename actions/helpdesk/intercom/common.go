// Package intercom holds the shared HTTP client, auth helpers, and generic
// resource helpers used by every helpdesk/intercom/* action.
//
// Intercom's REST API (pinned to version 2.15 via the Intercom-Version header)
// is uniform across resources — contacts, companies, conversations, tickets,
// tags, notes, segments, admins, teams, and articles follow conventional
// create/read/update/delete/list/search shapes under fixed regional hosts.
// That regularity lets the transport, auth, error handling and result-shaping
// live here once, so each action package stays thin: read its inputs, call one
// helper, shape the result — the same design as the sibling zendesk package.
//
// Five things shape this file:
//
//   - Auth is a private-app Access Token sent as an HTTP Bearer header. It is
//     a ConnectionTypeSecret (rendered as an env-secret picker) and is
//     scrubbed from any error string. The base host is FIXED per region
//     (api.intercom.io / api.eu.intercom.io / api.au.intercom.io), never
//     caller-supplied, so there is no SSRF surface and no insecure path to
//     gate; cross-host redirects are refused outright.
//
//   - Single resources come back as plain top-level objects (no envelope),
//     but LIST envelopes vary per resource ({"data": [...]} for contacts and
//     tags, {"conversations": [...]}, {"tickets": [...]}, {"admins": [...]},
//     ...). ListPage/ListAll/SearchAll take the array key explicitly and fall
//     back to "data", then to the single array-typed top-level key.
//
//   - Lists paginate with an opaque cursor: a page response carries
//     "pages": {"next": {"page": N, "starting_after": "..."}} (next is
//     null/absent on the last page). GET lists send the cursor as a
//     starting_after query param; POST searches carry a "pagination" object
//     INSIDE the JSON body — SearchAll owns that difference.
//
//   - A handful of endpoints (conversation/ticket untag) require a JSON body
//     on DELETE, so Do sends the body for DELETE as well as POST/PUT.
//
//   - POST /events replies 202 with an EMPTY body — decode treats an empty
//     body as an empty object rather than a parse failure.
package intercom

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
	// APIVersion is pinned on every request via the Intercom-Version header so
	// behaviour never shifts under a workspace's default-version setting.
	APIVersion = "2.15"

	// maxResponseBody caps the response body to prevent memory exhaustion.
	maxResponseBody = 8 << 20 // 8 MB

	requestTimeout = 30 * time.Second

	// DefaultPageLimit / MaxPageLimit bound a single list page. Intercom caps
	// per_page at 150 on cursor-paginated endpoints.
	DefaultPageLimit = 50
	MaxPageLimit     = 150

	// MaxAllPages bounds a "return all" pagination loop so a huge workspace
	// can never spin unbounded requests.
	MaxAllPages = 100
)

// regionHosts maps the region auth input to Intercom's fixed regional API
// hosts. The hosts are constants (never caller-supplied), so every request
// targets Intercom and there is no SSRF surface.
var regionHosts = map[string]string{
	"us": "https://api.intercom.io",
	"eu": "https://api.eu.intercom.io",
	"au": "https://api.au.intercom.io",
}

// httpClient is shared across every Intercom action so TLS connections to the
// regional API host are pooled and reused rather than re-dialled per call.
// Redirects to a different host are refused so the bearer token can never
// travel to a third party.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
	CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 0 && req.URL.Host != via[0].URL.Host {
			return fmt.Errorf("refusing cross-host redirect to %s", req.URL.Host)
		}
		if len(via) >= 5 {
			return fmt.Errorf("stopped after 5 redirects")
		}
		return nil
	},
}

// AuthInputs is the shared credential shape. Action packages re-declare their
// own literal Inputs arrays (the manifest generator AST-parses those), but this
// documents the canonical pair every action puts first.
//
// core.FindConnection returns the FIRST input whose name matches, so no
// resource field may be named api_token or region — a resource field sharing a
// credential's name would silently resolve to the credential.
var AuthInputs = []core.Connection{
	{
		Name:        "api_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Access Token",
		Placeholder: "Your Intercom access token (Developer Hub → Authentication)",
		Required:    true,
	},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
}

// Auth is the resolved credential: the access token plus the workspace's data
// region (us/eu/au), which selects the API host.
type Auth struct {
	Token  string
	Region string
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// GetAuth resolves the access token and region from the action's auth inputs.
// An empty region defaults to "us" (Intercom's US host also auto-routes, but
// the explicit regional host is what Intercom recommends).
func GetAuth(inputs []*core.Connection) (Auth, error) {
	token, err := RequiredString("api_token", inputs)
	if err != nil {
		return Auth{}, err
	}
	region := strings.ToLower(OptionalString("region", inputs))
	if region == "" {
		region = "us"
	}
	if _, ok := regionHosts[region]; !ok {
		return Auth{}, fmt.Errorf("region must be us, eu, or au (got %q)", region)
	}
	return Auth{Token: token, Region: region}, nil
}

// baseOverride, when non-empty, redirects every request to a test server
// regardless of region. Test-only seam.
var baseOverride = ""

// SetBaseForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores the real regional hosts. It
// lets action packages in sibling directories exercise Execute end-to-end
// without hitting Intercom. Test-only.
func SetBaseForTest(base string) func() {
	prev := baseOverride
	baseOverride = strings.TrimRight(base, "/")
	return func() { baseOverride = prev }
}

// BaseURL returns the API root for the auth's region.
func (a Auth) BaseURL() string {
	if baseOverride != "" {
		return baseOverride
	}
	if host, ok := regionHosts[a.Region]; ok {
		return host
	}
	return regionHosts["us"]
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// Do performs a REST call to Intercom. method is GET/POST/PUT/DELETE; path is
// the resource path under the regional API root beginning with "/". body, when
// non-nil, is marshalled as the JSON payload for POST/PUT — and for DELETE,
// which Intercom requires on conversation/ticket untag ({"admin_id": ...}).
// query carries GET filters and pagination; nil is fine.
func Do(a Auth, method, path string, body map[string]interface{}, query url.Values) (*APIResponse, error) {
	fullURL := a.BaseURL() + path
	if enc := query.Encode(); enc != "" {
		fullURL += "?" + enc
	}

	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodDelete) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", redactAuth(a, err.Error()))
	}
	req.Header.Set("Authorization", "Bearer "+a.Token)
	req.Header.Set("Intercom-Version", APIVersion)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Intercom API request failed: %s", redactAuth(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %s", redactAuth(a, err.Error()))
	}
	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// redactAuth removes the access token from an error message. It travels in the
// Authorization header (not the URL), so a leak is unlikely — but a wrapped
// error could echo it, so it is scrubbed defensively.
func redactAuth(a Auth, msg string) string {
	if a.Token != "" {
		msg = strings.ReplaceAll(msg, a.Token, "REDACTED")
	}
	return msg
}

// CheckResponse verifies a 2xx status, decoding Intercom's error envelope
// ({"type": "error.list", "errors": [{"code": "...", "message": "..."}]}).
// All errors are surfaced as "code: message", joined. The token never appears
// here — it travels only in the request header, never in a response body.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		if reset := resp.Headers.Get("X-RateLimit-Reset"); reset != "" {
			return fmt.Errorf("Intercom API rate limit exceeded (429); window resets at %s", reset)
		}
		return fmt.Errorf("Intercom API rate limit exceeded (429)")
	}

	var env struct {
		Type   string `json:"type"`
		Errors []struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"errors"`
	}
	parts := []string{}
	if err := json.Unmarshal(resp.Body, &env); err == nil {
		for _, e := range env.Errors {
			switch {
			case e.Code != "" && e.Message != "":
				parts = append(parts, e.Code+": "+e.Message)
			case e.Message != "":
				parts = append(parts, e.Message)
			case e.Code != "":
				parts = append(parts, e.Code)
			}
		}
	}
	if len(parts) > 0 {
		return fmt.Errorf("Intercom API error (%d): %s", resp.StatusCode, strings.Join(parts, "; "))
	}
	body := strings.TrimSpace(string(resp.Body))
	if len(body) > 500 {
		body = body[:500]
	}
	if body == "" {
		body = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("Intercom API error (%d): %s", resp.StatusCode, body)
}

// DecodeBody is the exported form of decodeObject, for the few actions (e.g.
// article search's nested data.articles, company lookup's classic envelope)
// that issue a request outside the generic helpers and need the raw success
// body themselves.
func DecodeBody(resp *APIResponse) (map[string]interface{}, error) {
	return decodeObject(resp)
}

// decodeObject unmarshals a successful response body into a generic map. An
// empty body yields an empty map — POST /events replies 202 with no content.
func decodeObject(resp *APIResponse) (map[string]interface{}, error) {
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Intercom response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Generic resource helpers
// ---------------------------------------------------------------------------

// GetObject GETs a single resource and decodes the response object.
func GetObject(a Auth, path string, query url.Values) (map[string]interface{}, error) {
	resp, err := Do(a, http.MethodGet, path, nil, query)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// WriteObject performs a POST or PUT with the given fields and decodes the
// returned object (Intercom echoes the created/updated resource).
func WriteObject(a Auth, method, path string, fields map[string]interface{}, query url.Values) (map[string]interface{}, error) {
	resp, err := Do(a, method, path, fields, query)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decodeObject(resp)
}

// DeleteResource DELETEs a resource. Intercom returns a small confirmation
// object ({"id": "...", "deleted": true}) on success.
func DeleteResource(a Auth, path string) error {
	resp, err := Do(a, http.MethodDelete, path, nil, nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// DeleteWithBody DELETEs a resource with a JSON payload — conversation and
// ticket untag require {"admin_id": ...} on the DELETE request.
func DeleteWithBody(a Auth, path string, body map[string]interface{}) error {
	resp, err := Do(a, http.MethodDelete, path, body, nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ListPage fetches a single page of a collection and returns the items plus
// the next-page cursor ("" when there are no more pages). arrayKey names the
// envelope's array ("data", "conversations", "admins", ...); when it misses,
// "data" is tried, then the single array-typed top-level key.
func ListPage(a Auth, path string, query url.Values, arrayKey string) ([]interface{}, string, error) {
	resp, err := Do(a, http.MethodGet, path, nil, query)
	if err != nil {
		return nil, "", err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, "", err
	}
	raw, err := decodeObject(resp)
	if err != nil {
		return nil, "", err
	}
	return extractItems(raw, arrayKey), nextCursor(raw), nil
}

// ListAll fetches a collection. When returnAll is false a single page (size
// limit, default 50) is returned; when true it follows the starting_after
// cursor at 150 per page to the MaxAllPages cap. Extra filters come in via
// query.
func ListAll(a Auth, path string, query url.Values, arrayKey string, limit int, returnAll bool) ([]interface{}, error) {
	if query == nil {
		query = url.Values{}
	}
	pageSize := ClampLimit(limit, limit > 0)
	if returnAll {
		pageSize = MaxPageLimit
	}
	query.Set("per_page", strconv.Itoa(pageSize))
	all := []interface{}{}
	pages := 0
	for {
		items, cursor, err := ListPage(a, path, query, arrayKey)
		if err != nil {
			return nil, err
		}
		all = append(all, items...)
		pages++
		if !returnAll || cursor == "" || len(items) == 0 || pages >= MaxAllPages {
			break
		}
		query.Set("starting_after", cursor)
	}
	return all, nil
}

// SearchAll POSTs a search (or cursor-list, e.g. /companies/list) request,
// following Intercom's body-carried pagination: {"pagination": {"per_page": N,
// "starting_after": cursor}} rides INSIDE the JSON payload, unlike GET lists.
// queryDSL, when non-nil, is sent as the "query" field (BuildSearchQuery
// produces it); /companies/list takes no query at all.
func SearchAll(a Auth, path string, queryDSL map[string]interface{}, arrayKey string, limit int, returnAll bool) ([]interface{}, error) {
	pageSize := ClampLimit(limit, limit > 0)
	if returnAll {
		pageSize = MaxPageLimit
	}
	pagination := map[string]interface{}{"per_page": pageSize}
	body := map[string]interface{}{"pagination": pagination}
	if queryDSL != nil {
		body["query"] = queryDSL
	}
	all := []interface{}{}
	pages := 0
	for {
		resp, err := Do(a, http.MethodPost, path, body, nil)
		if err != nil {
			return nil, err
		}
		if err := CheckResponse(resp); err != nil {
			return nil, err
		}
		raw, err := decodeObject(resp)
		if err != nil {
			return nil, err
		}
		items := extractItems(raw, arrayKey)
		all = append(all, items...)
		pages++
		cursor := nextCursor(raw)
		if !returnAll || cursor == "" || len(items) == 0 || pages >= MaxAllPages {
			break
		}
		pagination["starting_after"] = cursor
	}
	return all, nil
}

// extractItems pulls the item array out of a list envelope. Intercom's array
// key varies per resource, so the caller names it; "data" is the fallback,
// then the single array-typed top-level key (so an unexpected envelope still
// lists rather than silently returning nothing).
func extractItems(raw map[string]interface{}, arrayKey string) []interface{} {
	if arrayKey != "" {
		if arr, ok := raw[arrayKey].([]interface{}); ok {
			return arr
		}
	}
	if arr, ok := raw["data"].([]interface{}); ok {
		return arr
	}
	var found []interface{}
	count := 0
	for _, v := range raw {
		if arr, ok := v.([]interface{}); ok {
			found = arr
			count++
		}
	}
	if count == 1 {
		return found
	}
	return []interface{}{}
}

// nextCursor extracts the starting_after cursor from a list envelope's pages
// object. pages.next is an OBJECT ({"page": N, "starting_after": "..."}) on
// cursor endpoints and null/absent on the last page; anything else (e.g. a
// legacy next URL string) also means "no cursor".
func nextCursor(raw map[string]interface{}) string {
	pages, ok := raw["pages"].(map[string]interface{})
	if !ok {
		return ""
	}
	next, ok := pages["next"].(map[string]interface{})
	if !ok {
		return ""
	}
	cursor, _ := next["starting_after"].(string)
	return cursor
}

// ---------------------------------------------------------------------------
// Search query building
// ---------------------------------------------------------------------------

// searchOperators is Intercom's search DSL operator set. IN/NIN take array
// values; > and < are int/date only; ~ !~ ^ $ are string contains/not/starts/
// ends.
var searchOperators = map[string]bool{
	"=": true, "!=": true, "IN": true, "NIN": true,
	">": true, "<": true, "~": true, "!~": true, "^": true, "$": true,
}

// BuildSearchQuery assembles the "query" object for the search endpoints from
// the shared field/operator/value/query_json inputs. A query_json input wins
// verbatim (the power-user path for AND/OR compounds); otherwise a single
// filter is built, splitting the value on commas into an array for IN/NIN.
func BuildSearchQuery(inputs []*core.Connection) (map[string]interface{}, error) {
	raw, err := OptionalJSON("query_json", inputs)
	if err != nil {
		return nil, err
	}
	if raw != nil {
		obj, ok := raw.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf(`query_json must be a JSON object, e.g. {"field":"email","operator":"~","value":"@acme.com"}`)
		}
		return obj, nil
	}

	field := OptionalString("field", inputs)
	if field == "" {
		return nil, fmt.Errorf("provide a Field to filter on, or an Advanced Query (JSON)")
	}
	operator := OptionalString("operator", inputs)
	if operator == "" {
		operator = "="
	}
	if !searchOperators[operator] {
		return nil, fmt.Errorf("operator must be one of =, !=, IN, NIN, >, <, ~, !~, ^, $")
	}
	value := OptionalString("value", inputs)
	if operator == "IN" || operator == "NIN" {
		list := SplitCSV(value)
		vals := make([]interface{}, 0, len(list))
		for _, v := range list {
			vals = append(vals, v)
		}
		return map[string]interface{}{"field": field, "operator": operator, "value": vals}, nil
	}
	return map[string]interface{}{"field": field, "operator": operator, "value": value}, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// OptionalString extracts a string input, returning "" if absent.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}

// RequiredString extracts a required string input, erroring if absent/blank.
func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

// OptionalInt extracts an integer input. The bool is false when absent, so
// callers distinguish "unset" from "set to 0". Accepts both Integer-typed
// inputs and numeric text typed into a String input ("150").
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil {
		return 0, false
	}
	if n := conn.Number(); n != nil {
		return int(*n), true
	}
	if s := conn.String(); s != nil {
		if v, err := strconv.Atoi(strings.TrimSpace(*s)); err == nil {
			return v, true
		}
	}
	return 0, false
}

// OptionalBoolSet extracts a boolean input. set is false when absent, so
// callers preserve the tri-state nil as "omit".
func OptionalBoolSet(name string, inputs []*core.Connection) (val, set bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false, false
	}
	return *conn.Boolean(), true
}

// OptionalJSON parses an object/array-typed input into an arbitrary value.
// Used for the nested structures Intercom takes (custom_attributes,
// ticket_attributes, event metadata, and the additional_fields escape hatch).
// Returns (nil, nil) when the input is absent/blank, (nil, err) on malformed
// JSON so the action can surface a clear message.
func OptionalJSON(name string, inputs []*core.Connection) (interface{}, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return nil, nil
	}
	// Object/array inputs may arrive already-parsed (from ${...} wiring) or as
	// a JSON string typed into the editor.
	switch v := conn.Value.(type) {
	case string:
		if strings.TrimSpace(v) == "" {
			return nil, nil
		}
		var out interface{}
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, fmt.Errorf("%s must be valid JSON: %w", name, err)
		}
		return out, nil
	default:
		return conn.Value, nil
	}
}

// SplitCSV turns a comma-separated string ("vip, wholesale") into a trimmed,
// non-empty slice (["vip","wholesale"]) — used for attachment_urls and IN/NIN
// search values, which Intercom takes as arrays. Returns nil for an empty
// input.
func SplitCSV(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SetIfPresent adds an optional string field to a resource body only when the
// input was provided, so unset fields are omitted.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// SetCSVIfPresent adds an optional comma-separated input as a string array
// field only when non-empty (attachment_urls).
func SetCSVIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := SplitCSV(OptionalString(inputName, inputs)); v != nil {
		body[field] = v
	}
}

// SetIntIfPresent adds an optional integer field when its input is present, so
// the unset case is omitted rather than sent as 0. A value that is present but
// not numeric is an ERROR rather than a silent omission — the same contract as
// the date fields (SetUnixIfPresent) — so a wired "1,200" or "n/a" can't make
// the action report success while the field never reached Intercom. Decimal
// values are truncated toward zero, matching Intercom's documented handling of
// integer fields like monthly_spend.
func SetIntIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) error {
	conn := core.FindConnection(inputName, inputs)
	if conn == nil || conn.Value == nil {
		return nil
	}
	if n := conn.Number(); n != nil {
		body[field] = int(*n)
		return nil
	}
	// Read the raw value directly: Connection.String() refuses unparseable
	// Integer-typed values, which is exactly the case that must be reported
	// rather than dropped.
	raw := strings.TrimSpace(fmt.Sprintf("%v", conn.Value))
	if raw == "" {
		return nil
	}
	if v, err := strconv.Atoi(raw); err == nil {
		body[field] = v
		return nil
	}
	if f, err := strconv.ParseFloat(raw, 64); err == nil {
		body[field] = int(f)
		return nil
	}
	return fmt.Errorf("%s must be a whole number (got %q)", inputName, raw)
}

// SetNumericIDIfPresent adds an optional ID field, sent as a number when the
// value parses as an integer. Intercom types some ID fields as JSON integers
// (owner_id, author_id, message from.id) and rejects string values there,
// while dropdown/entered values always arrive as strings. A non-numeric value
// is passed through as a string rather than dropped, so an unexpected format
// still reaches Intercom to surface its own error.
func SetNumericIDIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	v := OptionalString(inputName, inputs)
	if v == "" {
		return
	}
	if n, err := strconv.ParseInt(v, 10, 64); err == nil {
		body[field] = n
		return
	}
	body[field] = v
}

// SetBoolIfSet adds an optional boolean field when its input connection is
// present (checkbox touched), so the tri-state nil is preserved as "omit".
func SetBoolIfSet(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v, set := OptionalBoolSet(inputName, inputs); set {
		body[field] = v
	}
}

// SetJSONIfPresent parses an optional JSON input and adds it to the body when
// present. Returns an error only on malformed JSON.
func SetJSONIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) error {
	v, err := OptionalJSON(inputName, inputs)
	if err != nil {
		return err
	}
	if v != nil {
		body[field] = v
	}
	return nil
}

// SetUnixIfPresent adds an optional date/time input as Unix epoch seconds —
// Intercom timestamps (signed_up_at, last_seen_at, created_at, snoozed_until,
// remote_created_at) are all epoch-typed. Returns an error on an
// unrecognisable value.
func SetUnixIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) error {
	v := OptionalString(inputName, inputs)
	if v == "" {
		return nil
	}
	n, err := ParseUnixTime(v)
	if err != nil {
		return fmt.Errorf("%s: %s", inputName, err)
	}
	body[field] = n
	return nil
}

// maxEpochSeconds is the magnitude above which a bare integer timestamp can
// only be a millisecond epoch: 100_000_000_000 seconds is the year ~5138, so
// no sane seconds value ever exceeds it, while every JS-style Date.now() value
// (13 digits) does.
const maxEpochSeconds = 100_000_000_000

// ParseUnixTime converts a date/time input to Unix epoch seconds. DateTime
// inputs arrive as strings, and users wire in everything from ISO stamps to
// raw epochs, so it is liberal: epoch seconds ("1720000000"), epoch
// milliseconds ("1783534378000", the JS Date.now() shape — converted to
// seconds), RFC3339 ("2026-07-08T09:00:00Z"), a zoneless stamp (taken as
// UTC), or a bare date ("2026-07-08" → midnight UTC).
func ParseUnixTime(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty date/time")
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		// A millisecond epoch taken as seconds lands in year ~58,000 — and
		// Intercom's async validation (e.g. POST /events' fire-and-forget 202)
		// swallows such values without ever surfacing an error. Anything beyond
		// maxEpochSeconds in magnitude is unambiguously milliseconds.
		if n > maxEpochSeconds || n < -maxEpochSeconds {
			n /= 1000
		}
		return n, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05", "2006-01-02 15:04:05", "2006-01-02"} {
		if t, err := time.Parse(layout, value); err == nil {
			return t.Unix(), nil
		}
	}
	return 0, fmt.Errorf("%q is not a recognised date/time — use RFC3339 (2026-07-08T09:00:00Z), a date (2026-07-08), or Unix seconds", value)
}

// MergeAdditionalFields overlays a raw JSON object input ("additional_fields")
// onto the resource body — the escape hatch for any Intercom field not exposed
// as a first-class input. Called LAST so a key here OVERRIDES a first-class
// input (the "power-user last word" precedence shared with the sibling nodes).
func MergeAdditionalFields(body map[string]interface{}, inputs []*core.Connection) error {
	v, err := OptionalJSON("additional_fields", inputs)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Errorf(`additional_fields must be a JSON object, e.g. {"key":"value"}`)
	}
	for k, val := range obj {
		body[k] = val
	}
	return nil
}

// ClampLimit bounds a requested page size to Intercom's 1-150 range, falling
// back to DefaultPageLimit when unset.
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

// ErrorResult is the standard soft-failure output map (returned with a nil
// error so the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ResourceResult shapes a single-resource response (create/get/update) into
// the standard action output. The id output reads the resource's "id".
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          StringifyID(obj["id"]),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// SuccessResult shapes an operation whose id the caller already knows (delete /
// tag / untag side calls that return an empty or confirmation body).
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

// ListResult shapes a collection response into the standard list output.
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

// StringifyID renders Intercom's mixed ID types (opaque strings for contacts
// and conversations, JSON numbers for admins, teams and tags) as a clean
// string.
func StringifyID(id interface{}) string {
	switch v := id.(type) {
	case nil:
		return ""
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	case int64:
		return strconv.FormatInt(v, 10)
	default:
		return fmt.Sprintf("%v", v)
	}
}
