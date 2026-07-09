// Package sendgrid holds the shared HTTP client, auth helpers, and generic
// resource helpers used by every marketing/sendgrid/* action.
//
// SendGrid's v3 REST API spans two families under one host: the transactional
// endpoints (mail send, templates, suppressions, ASM groups, stats) and the
// newer Marketing endpoints (/v3/marketing/* — contacts, lists, segments).
// Auth is a single API key sent as an HTTP Bearer header against a FIXED host
// per region (api.sendgrid.com globally, api.eu.sendgrid.com for EU
// data-residency subusers), so there is no SSRF surface; cross-host redirects
// are refused outright.
//
// Six things shape this file:
//
//   - Two pagination families. Marketing and template lists take page_size
//     (+page_token) and answer with a "_metadata" envelope whose "next" field
//     is a full URL — ListMarketing follows it by extracting the URL's QUERY
//     and re-issuing against our fixed host and path, never fetching the next
//     URL itself. Legacy suppression lists take limit (1-500) and offset with
//     no metadata at all — ListOffset stops when a page comes back short.
//
//   - Several endpoints return a top-level JSON ARRAY, not an object (legacy
//     suppression collections, ASM groups, ASM group suppressions, stats), so
//     Do decodes into interface{} and callers assert the shape they expect.
//
//   - POST /v3/mail/send answers 202 with an EMPTY body and the message id in
//     the X-Message-Id response HEADER — Do therefore returns the response
//     headers and status alongside the decoded body, and decodes an empty
//     body as an empty object rather than a parse failure.
//
//   - Marketing writes are ASYNCHRONOUS: contact upserts/deletes answer
//     202 {"job_id": ...} and a marketing list delete answers either
//     200 {"job_id": ...} or 204 empty. Callers surface the job id and say so.
//
//   - The error envelope is {"errors":[{"message","field"}]} on transactional
//     endpoints, with a marketing variant carrying "error_id"/"parameter";
//     429 responses carry X-RateLimit-Reset. CheckResponse decodes all of
//     them, and Do scrubs the API key from every error string.
//
//   - The EU host has NO Marketing endpoints (contacts/lists/segments are
//     unavailable for EU-pinned subusers), and an EU subuser's key fails on
//     the global host — the region input's description says so.
package sendgrid

import (
	"bytes"
	"encoding/json"
	"errors"
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
	// maxResponseBody caps the response body to prevent memory exhaustion.
	maxResponseBody = 8 << 20 // 8 MB

	requestTimeout = 30 * time.Second

	// DefaultPageLimit / MaxPageLimit bound a single list page. SendGrid's
	// marketing endpoints cap page_size at 1000; legacy suppression lists cap
	// limit at 500 and template lists cap page_size at 200.
	DefaultPageLimit        = 100
	MaxPageLimit            = 1000
	MaxSuppressionPageLimit = 500
	MaxTemplatePageSize     = 200

	// MaxAllPages bounds a "return all" pagination loop so a huge account can
	// never spin unbounded requests.
	MaxAllPages = 100
)

// regionHosts maps the region auth input to SendGrid's fixed API hosts. The
// hosts are constants (never caller-supplied), so every request targets
// SendGrid and there is no SSRF surface. The empty region is the global host;
// "eu" is only for EU data-residency subusers (whose keys fail on the global
// host) and lacks the /v3/marketing/* endpoints entirely.
var regionHosts = map[string]string{
	"":   "https://api.sendgrid.com",
	"eu": "https://api.eu.sendgrid.com",
}

// hostFor returns the fixed API host for a region, defaulting to the global
// host for anything unrecognised (GetAuth validates the region up front).
func hostFor(region string) string {
	if host, ok := regionHosts[region]; ok {
		return host
	}
	return regionHosts[""]
}

// httpClient is shared across every SendGrid action so TLS connections to the
// API host are pooled and reused rather than re-dialled per call. Redirects to
// a different host are refused so the API key can never travel to a third
// party.
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
// resource field may be named api_key or region — a resource field sharing a
// credential's name would silently resolve to the credential.
var AuthInputs = []core.Connection{
	{
		Name:        "api_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "API Key",
		Placeholder: "SendGrid API key (SendGrid → Settings → API Keys), e.g. ${secrets.sendgrid_api}",
		Required:    true,
	},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "Global", Value: ""},
			{Name: "EU (data residency)", Value: "eu"},
		},
		Placeholder: "Global unless your account uses an EU regional subuser — the EU host has no Marketing endpoints (contacts, lists, segments)",
	},
}

// Auth is the resolved credential: the API key plus the account's region
// ("" for global, "eu" for an EU data-residency subuser), which selects the
// API host.
type Auth struct {
	APIKey string
	Region string
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// GetAuth resolves the API key and region from the action's auth inputs. An
// empty region is the global host; only "eu" selects the EU data-residency
// host.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	key, err := RequiredString("api_key", inputs)
	if err != nil {
		return Auth{}, err
	}
	region := strings.ToLower(OptionalString("region", inputs))
	if _, ok := regionHosts[region]; !ok {
		return Auth{}, fmt.Errorf("region must be blank (Global) or eu (got %q)", region)
	}
	return Auth{APIKey: key, Region: region}, nil
}

// baseOverride, when non-empty, redirects every request to a test server
// regardless of region. Test-only seam.
var baseOverride = ""

// SetBaseForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores the real hosts. It lets action
// packages in sibling directories exercise Execute end-to-end without hitting
// SendGrid. Test-only.
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
	return hostFor(a.Region)
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// Do performs a REST call to SendGrid and returns the decoded JSON body (a
// map for object responses, a []interface{} for the endpoints that answer
// with a top-level array, an empty map for empty 202/204 bodies), the response
// headers (mail_send reads X-Message-Id), and the status code (list_delete
// distinguishes 200 {"job_id"} from 204 empty). method is
// GET/POST/PUT/PATCH/DELETE; path is the resource path under the API root
// beginning with "/". body, when non-nil, is marshalled as the JSON payload —
// including for DELETE, which the bulk suppression endpoints require
// ({"emails": [...]} / {"delete_all": true}). query carries GET filters and
// pagination; nil is fine. A non-2xx status is returned as an error with the
// API key redacted.
func Do(a Auth, method, path string, query url.Values, body interface{}) (interface{}, http.Header, int, error) {
	fullURL := a.BaseURL() + path
	if enc := query.Encode(); enc != "" {
		fullURL += "?" + enc
	}

	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPut || method == http.MethodPatch || method == http.MethodDelete) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, 0, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to create request: %s", redactAuth(a, err.Error()))
	}
	req.Header.Set("Authorization", "Bearer "+a.APIKey)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("SendGrid API request failed: %s", redactAuth(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to read response: %s", redactAuth(a, err.Error()))
	}
	apiResp := &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}
	if err := CheckResponse(apiResp); err != nil {
		return nil, resp.Header, resp.StatusCode, errors.New(redactAuth(a, err.Error()))
	}
	result, err := decodeJSON(respBody)
	if err != nil {
		return nil, resp.Header, resp.StatusCode, err
	}
	return result, resp.Header, resp.StatusCode, nil
}

// redactAuth removes the API key from an error message. It travels in the
// Authorization header (not the URL), so a leak is unlikely — but a wrapped
// error or an echoed response body could contain it, so every error string is
// scrubbed defensively.
func redactAuth(a Auth, msg string) string {
	if a.APIKey != "" {
		msg = strings.ReplaceAll(msg, a.APIKey, "REDACTED")
	}
	return msg
}

// CheckResponse verifies a 2xx status, decoding SendGrid's error envelope
// ({"errors":[{"message","field"}]}, with the marketing variant carrying
// "error_id" and "parameter" instead of "field"). Errors are surfaced as
// "field: message", joined. A 429 includes the X-RateLimit-Reset header so the
// user knows when the window reopens.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		if reset := resp.Headers.Get("X-RateLimit-Reset"); reset != "" {
			return fmt.Errorf("SendGrid API rate limit exceeded (429); window resets at %s", reset)
		}
		return fmt.Errorf("SendGrid API rate limit exceeded (429)")
	}

	var env struct {
		Errors []struct {
			Message   string `json:"message"`
			Field     string `json:"field"`
			ErrorID   string `json:"error_id"`
			Parameter string `json:"parameter"`
		} `json:"errors"`
	}
	parts := []string{}
	if err := json.Unmarshal(resp.Body, &env); err == nil {
		for _, e := range env.Errors {
			label := e.Field
			if label == "" {
				label = e.Parameter
			}
			switch {
			case label != "" && e.Message != "":
				parts = append(parts, label+": "+e.Message)
			case e.Message != "":
				parts = append(parts, e.Message)
			case label != "":
				parts = append(parts, label)
			case e.ErrorID != "":
				parts = append(parts, e.ErrorID)
			}
		}
	}
	if len(parts) > 0 {
		return fmt.Errorf("SendGrid API error (%d): %s", resp.StatusCode, strings.Join(parts, "; "))
	}
	body := strings.TrimSpace(string(resp.Body))
	if len(body) > 500 {
		body = body[:500]
	}
	if body == "" {
		body = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("SendGrid API error (%d): %s", resp.StatusCode, body)
}

// decodeJSON unmarshals a successful response body. An empty body yields an
// empty map — mail send answers 202 with no content, and several DELETEs
// answer 204. The result stays an interface{} because legacy suppression
// collections, ASM groups, and stats answer with a TOP-LEVEL ARRAY rather
// than an object.
func decodeJSON(body []byte) (interface{}, error) {
	if len(bytes.TrimSpace(body)) == 0 {
		return map[string]interface{}{}, nil
	}
	var out interface{}
	if err := json.Unmarshal(body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse SendGrid response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Pagination
// ---------------------------------------------------------------------------

// ListMarketing fetches a marketing-family collection (marketing lists,
// segments, templates) that paginates with page_size/page_token and a
// "_metadata" envelope. When returnAll is false a single page (size limit,
// default 100) is returned; when true it follows "_metadata.next" to the
// MaxAllPages cap — by extracting the next URL's QUERY STRING and re-issuing
// against OUR fixed host and path, never fetching the next URL itself.
// arrayKey names the envelope's array ("result" for lists/segments/templates,
// "results" for verified senders); when it misses, "result", "results", and
// the single array-typed top-level key are tried in turn. Template lists cap
// page_size at 200 (and the API requires the parameter — it is always sent);
// a page_size already present in query wins over the computed one.
func ListMarketing(a Auth, path string, query url.Values, arrayKey string, limit int, returnAll bool) ([]interface{}, error) {
	if query == nil {
		query = url.Values{}
	}
	maxPage := MaxPageLimit
	if strings.HasPrefix(path, "/v3/templates") {
		maxPage = MaxTemplatePageSize
	}
	if query.Get("page_size") == "" {
		pageSize := ClampLimit(limit, limit > 0, DefaultPageLimit, maxPage)
		if returnAll {
			pageSize = maxPage
		}
		query.Set("page_size", strconv.Itoa(pageSize))
	}
	all := []interface{}{}
	pages := 0
	for {
		result, _, _, err := Do(a, http.MethodGet, path, query, nil)
		if err != nil {
			return nil, err
		}
		raw, ok := result.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected SendGrid list response shape")
		}
		items := extractItems(raw, arrayKey)
		all = append(all, items...)
		pages++
		next := metadataNext(raw)
		if !returnAll || next == "" || len(items) == 0 || pages >= MaxAllPages {
			break
		}
		nq := nextPageQuery(next)
		if nq == nil {
			break
		}
		query = nq
	}
	return all, nil
}

// metadataNext extracts the next-page URL from a marketing envelope's
// "_metadata" object; "" means last page.
func metadataNext(raw map[string]interface{}) string {
	meta, ok := raw["_metadata"].(map[string]interface{})
	if !ok {
		return ""
	}
	next, _ := meta["next"].(string)
	return next
}

// nextPageQuery turns a "_metadata.next" URL into the query values for the
// next request. Only the URL's query string is trusted — the request itself
// is re-issued against our fixed host and path.
func nextPageQuery(next string) url.Values {
	u, err := url.Parse(next)
	if err != nil {
		return nil
	}
	q, err := url.ParseQuery(u.RawQuery)
	if err != nil || len(q) == 0 {
		return nil
	}
	return q
}

// ListOffset fetches a legacy suppression collection (bounces, blocks, spam
// reports, invalid emails, global unsubscribes) that answers with a TOP-LEVEL
// JSON ARRAY and paginates with limit (1-500) and offset. When returnAll is
// false a single page (size limit, default 100) is returned; when true it
// pages at 500, stopping when a page comes back shorter than requested (the
// API sends no pagination metadata at all), on an empty page, or at the
// MaxAllPages cap.
func ListOffset(a Auth, path string, query url.Values, limit int, returnAll bool) ([]interface{}, error) {
	if query == nil {
		query = url.Values{}
	}
	pageSize := ClampLimit(limit, limit > 0, DefaultPageLimit, MaxSuppressionPageLimit)
	if returnAll {
		pageSize = MaxSuppressionPageLimit
	}
	query.Set("limit", strconv.Itoa(pageSize))
	offset := 0
	all := []interface{}{}
	pages := 0
	for {
		query.Set("offset", strconv.Itoa(offset))
		result, _, _, err := Do(a, http.MethodGet, path, query, nil)
		if err != nil {
			return nil, err
		}
		items, ok := result.([]interface{})
		if !ok {
			return nil, fmt.Errorf("unexpected SendGrid suppression response shape")
		}
		all = append(all, items...)
		pages++
		if !returnAll || len(items) < pageSize || pages >= MaxAllPages {
			break
		}
		offset += len(items)
	}
	return all, nil
}

// extractItems pulls the item array out of a list envelope. Marketing
// endpoints key it "result", verified senders "results", so the caller names
// it; both are fallbacks, then the single array-typed top-level key (so an
// unexpected envelope still lists rather than silently returning nothing).
func extractItems(raw map[string]interface{}, arrayKey string) []interface{} {
	for _, key := range []string{arrayKey, "result", "results"} {
		if key == "" {
			continue
		}
		if arr, ok := raw[key].([]interface{}); ok {
			return arr
		}
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
// inputs and numeric text typed into a String input ("500") — the limit
// inputs are deliberately ConnectionTypeString so ${...} wiring works.
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
// Used for the nested structures SendGrid takes (dynamic_template_data,
// attachments, custom_fields, custom_args, and the additional_fields escape
// hatch). Returns (nil, nil) when the input is absent/blank, (nil, err) on
// malformed JSON so the action can surface a clear message.
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

// SplitCSV turns a comma-separated string ("a@x.com, b@y.com") into a
// trimmed, non-empty slice — used for recipient lists, contact ids, and
// categories, which SendGrid takes as arrays. Returns nil for an empty input.
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
// field only when non-empty (categories, alternate_emails, list_ids).
func SetCSVIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := SplitCSV(OptionalString(inputName, inputs)); v != nil {
		body[field] = v
	}
}

// SetIntIfPresent adds an optional integer field when its input is present, so
// the unset case is omitted rather than sent as 0. A value that is present but
// not numeric is an ERROR rather than a silent omission — so a wired "1,200"
// or "n/a" can't make the action report success while the field never reached
// SendGrid. Decimal values are truncated toward zero.
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

// AddFilter sets a query parameter from an optional input only when the input
// is non-empty, so unset filters are omitted.
func AddFilter(q url.Values, inputs []*core.Connection, param, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		q.Set(param, v)
	}
}

// MergeAdditionalFields overlays a raw JSON object input ("additional_fields")
// onto the resource body — the escape hatch for any SendGrid field not exposed
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

// ClampLimit bounds a requested page size to the 1-max range, falling back to
// def when unset. SendGrid's page caps vary per endpoint family (1000 for
// marketing, 500 for suppressions, 200 for templates), so both bounds are the
// caller's.
func ClampLimit(limit int, set bool, def, max int) int {
	if !set || limit <= 0 {
		return def
	}
	if limit > max {
		return max
	}
	return limit
}

// maxEpochSeconds is the magnitude above which a bare integer timestamp can
// only be a millisecond epoch: 100_000_000_000 seconds is the year ~5138, so
// no sane seconds value ever exceeds it, while every JS-style Date.now() value
// (13 digits) does.
const maxEpochSeconds = 100_000_000_000

// EpochSeconds converts a date/time input to Unix epoch seconds — SendGrid's
// send_at and the suppression start_time/end_time filters are all
// epoch-typed. DateTime inputs arrive as strings, and users wire in
// everything from ISO stamps to raw epochs, so it is liberal: epoch seconds
// ("1720000000"), epoch milliseconds ("1783534378000", the JS Date.now()
// shape — converted to seconds), RFC3339 ("2026-07-10T09:00:00Z"), a zoneless
// stamp (taken as UTC), or a bare date ("2026-07-10" → midnight UTC).
func EpochSeconds(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, fmt.Errorf("empty date/time")
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
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
	return 0, fmt.Errorf("%q is not a recognised date/time — use RFC3339 (2026-07-10T09:00:00Z), a date (2026-07-10), or Unix seconds", value)
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

// ResourceResult shapes a single-resource response into the standard action
// output. SendGrid's id lives in different places per endpoint (the resource's
// "id", a job_id, the X-Message-Id header), so the caller names it.
func ResourceResult(id string, obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          id,
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// SuccessResult shapes an operation with no response body (the 204 deletes),
// whose id the caller already knows.
func SuccessResult(id, summary string) map[string]interface{} {
	return map[string]interface{}{
		"id":          id,
		"result":      map[string]interface{}{},
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output. The
// count is the caller's — usually len(results), but the contact search
// endpoints report a total match count beyond the 50 rows they return.
func ListResult(results []interface{}, count int, summary string) map[string]interface{} {
	if results == nil {
		results = []interface{}{}
	}
	return map[string]interface{}{
		"results":     results,
		"count":       count,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// StringifyID renders SendGrid's mixed ID types (uuid strings for lists and
// templates, JSON numbers for ASM groups) as a clean string.
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
