// Package woocommerce holds the shared HTTP client, auth helpers, and generic
// resource CRUD used by every ecommerce/woocommerce/* action.
//
// WooCommerce's REST API (v3) is uniform across resources — customers, orders,
// products and coupons all share identical create/read/update/delete/list
// shapes under /wp-json/wc/v3/{resource}[/{id}]. That regularity lets the CRUD
// live here once (CreateResource, GetResource, UpdateResource, DeleteResource,
// ListResources) so each action package stays thin: read its inputs, call one
// helper, shape the result — the same design as the sibling Shopify package.
//
// Two things differ from Shopify and shape this file:
//
//   - Responses are NOT enveloped. A single resource comes back as the bare
//     object ({"id":123,...}); a collection as a bare JSON array. So decode
//     yields a map and decodeList an []interface{}, with no {"order":{...}}
//     unwrapping.
//   - Pagination is page/per_page (WordPress style) with a rel="next" Link
//     header and X-WP-Total / X-WP-TotalPages count headers, rather than
//     Shopify's opaque page_info cursor.
//
// Auth is a WooCommerce REST API key pair — a Consumer Key + Consumer Secret,
// sent as HTTP Basic auth over HTTPS (the modern default). Some hosts strip the
// Authorization header (typically behind certain proxies, or over plain HTTP),
// so a "credentials in query" fallback puts the pair in the query string
// instead — mirroring WooCommerce's own documented workaround and n8n's
// includeCredentialsInQuery toggle. Both key parts are ConnectionTypeSecret;
// the store URL a ConnectionTypeString.
package woocommerce

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
	// APIBasePath is the WooCommerce REST API v3 prefix appended to the store
	// URL. Pinned to v3 (a single constant, not a per-action input) to keep the
	// UI clean for non-technical users — matching the shopify.APIVersion choice.
	APIBasePath = "/wp-json/wc/v3"

	// maxResponseBody caps the response body to prevent memory exhaustion. List
	// pages of orders/products can be large, so 8 MB (the shopify/airtable value)
	// rather than the 1 MB used by single-record integrations.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single WooCommerce call.
	requestTimeout = 30 * time.Second

	// MaxAllPages bounds a "return all" pagination loop so a store with a huge
	// catalogue can never spin unbounded requests. On hitting the cap the action
	// reports it and returns the next page number so the caller can resume.
	MaxAllPages = 100

	// DefaultPageLimit / MaxPageLimit bound WooCommerce's per_page (1-100).
	DefaultPageLimit = 50
	MaxPageLimit     = 100
)

// httpClient is shared across every WooCommerce action so TCP connections to the
// store are pooled and reused rather than re-dialled per call (a flow run — or a
// return-all loop — can fire many requests). Matches the connection-reuse
// pattern used by the Shopify and HubSpot integrations.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AuthInputs is the shared credential shape. Action packages re-declare their
// own literal Inputs arrays (the manifest generator AST-parses those), but this
// documents the canonical set every action puts first.
var AuthInputs = []core.Connection{
	{
		Name:        "url",
		Type:        core.ConnectionTypeString,
		Label:       "Store URL",
		Placeholder: "https://your-store.com",
		Required:    true,
	},
	{
		Name:        "consumer_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "Consumer Key",
		Placeholder: "ck_... (WooCommerce ▸ Settings ▸ Advanced ▸ REST API)",
		Required:    true,
	},
	{
		Name:        "consumer_secret",
		Type:        core.ConnectionTypeSecret,
		Label:       "Consumer Secret",
		Placeholder: "cs_...",
		Required:    true,
	},
	{
		Name:        "credentials_in_query",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Send Credentials in Query String",
		Placeholder: "Enable only if you see a \"Consumer key is missing\" error (server strips the auth header)",
	},
}

// Auth is the resolved connection: a normalised base URL (scheme + host [+ any
// subdirectory], no trailing slash, no /wp-json suffix) and the key pair. InQuery
// switches the credentials from a Basic auth header to query-string parameters.
type Auth struct {
	BaseURL        string
	ConsumerKey    string
	ConsumerSecret string
	InQuery        bool
}

// APIResponse wraps the HTTP response for consistent handling. Headers is
// carried because WooCommerce's pagination cursor (rel="next") and total counts
// (X-WP-Total / X-WP-TotalPages) live in response headers.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ---------------------------------------------------------------------------
// Auth resolution
// ---------------------------------------------------------------------------

// GetAuth resolves the store URL and Consumer Key/Secret from the action's auth
// inputs. A missing URL or either key part is a hard failure (zero Auth + real
// error) rather than a soft error output — there is nothing to attempt without
// them.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	rawURL, err := RequiredString("url", inputs)
	if err != nil {
		return Auth{}, err
	}
	base, err := NormaliseBaseURL(rawURL)
	if err != nil {
		return Auth{}, err
	}
	key, err := RequiredString("consumer_key", inputs)
	if err != nil {
		return Auth{}, err
	}
	secret, err := RequiredString("consumer_secret", inputs)
	if err != nil {
		return Auth{}, err
	}
	return Auth{
		BaseURL:        base,
		ConsumerKey:    key,
		ConsumerSecret: secret,
		InQuery:        OptionalBool("credentials_in_query", inputs),
	}, nil
}

// NormaliseBaseURL reduces whatever the user pasted to a clean scheme+host[+path]
// base with no trailing slash and no REST-API suffix. Accepts "store.com",
// "https://store.com", "https://store.com/", or a value that already includes
// "/wp-json/wc/v3". A subdirectory WordPress install (e.g. "store.com/shop") is
// preserved. Defaults to https:// when no scheme is given — the REST key pair is
// only safe over TLS.
func NormaliseBaseURL(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("url is required")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("url is not a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("url must be an http(s) URL, e.g. https://your-store.com")
	}
	if u.Host == "" {
		return "", fmt.Errorf("url must include a host, e.g. https://your-store.com")
	}
	// Drop the REST-API suffix if the user pasted a full endpoint URL, then trim
	// any trailing slash so BuildURL can append the path cleanly.
	path := strings.TrimRight(u.Path, "/")
	for _, suffix := range []string{APIBasePath, "/wp-json/wc/v2", "/wp-json/wc/v1", "/wp-json"} {
		if strings.HasSuffix(path, suffix) {
			path = strings.TrimSuffix(path, suffix)
			break
		}
	}
	return u.Scheme + "://" + u.Host + path, nil
}

// apiBase assembles the REST API root for a store. It is a var (not inline) so
// SetBaseForTest can point every request at an httptest server — the same seam
// idiom as the shopify package's hostForShop.
var apiBase = func(a Auth) string {
	return strings.TrimRight(a.BaseURL, "/") + APIBasePath
}

// SetBaseForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores the real base. It lets action
// packages in sibling directories exercise Execute end-to-end without a live
// store. Test-only.
func SetBaseForTest(base string) func() {
	prev := apiBase
	apiBase = func(Auth) string { return strings.TrimRight(base, "/") }
	return func() { apiBase = prev }
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// ExecuteAPI performs a REST call to the store's WooCommerce API.
// method: GET, POST, PUT, DELETE
// path:   resource path including any query string (e.g. "/orders/123?force=true")
// body:   optional payload — marshalled to JSON for POST/PUT, ignored otherwise
func ExecuteAPI(a Auth, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := apiBase(a) + path

	// Credentials-in-query fallback: append the key pair as query params. Merge
	// with any existing query string on the path.
	if a.InQuery {
		sep := "?"
		if strings.Contains(fullURL, "?") {
			sep = "&"
		}
		q := url.Values{}
		q.Set("consumer_key", a.ConsumerKey)
		q.Set("consumer_secret", a.ConsumerSecret)
		fullURL += sep + q.Encode()
	}

	var bodyReader io.Reader
	if body != nil && (method == http.MethodPost || method == http.MethodPut) {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, fullURL, bodyReader)
	if err != nil {
		// In credentials-in-query mode fullURL carries the key pair, and a
		// url.Error echoes it — scrub before returning (this error becomes a
		// node's visible/persisted output via ErrorResult).
		return nil, fmt.Errorf("failed to create request: %s", redactAuth(a, err.Error()))
	}

	if !a.InQuery {
		req.SetBasicAuth(a.ConsumerKey, a.ConsumerSecret)
	}
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		// The transport's *url.Error includes the request URL — which in
		// credentials-in-query mode holds the consumer key/secret. Scrub them.
		return nil, fmt.Errorf("WooCommerce API request failed: %s", redactAuth(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// redactAuth removes the consumer key/secret from an error message. In
// credentials-in-query mode they are carried in the request URL, which the
// net/http transport echoes into its errors — and those errors surface in a
// node's user-visible/persisted output via ErrorResult. Both the raw and the
// URL-query-escaped forms are scrubbed. A no-op when the values aren't present
// (Basic-auth mode keeps them out of the URL entirely).
func redactAuth(a Auth, msg string) string {
	for _, s := range []string{a.ConsumerSecret, a.ConsumerKey} {
		if s == "" {
			continue
		}
		msg = strings.ReplaceAll(msg, url.QueryEscape(s), "REDACTED")
		msg = strings.ReplaceAll(msg, s, "REDACTED")
	}
	return msg
}

// CheckResponse verifies a 2xx status, decoding WooCommerce's error envelope
// ({"code","message","data":{"status"}}). The message is the human-readable
// reason; code is appended when present so an unfamiliar error is greppable
// against the WooCommerce docs.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var env struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Body, &env); err == nil && env.Message != "" {
		if env.Code != "" {
			return fmt.Errorf("WooCommerce API error (%d): %s [%s]", resp.StatusCode, env.Message, env.Code)
		}
		return fmt.Errorf("WooCommerce API error (%d): %s", resp.StatusCode, env.Message)
	}
	body := string(resp.Body)
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Errorf("WooCommerce API error (%d): %s", resp.StatusCode, body)
}

// decode unmarshals a successful single-resource body into a generic map. An
// empty body yields an empty map.
func decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse WooCommerce response: %w", err)
	}
	return out, nil
}

// decodeList unmarshals a successful collection body into a generic slice. A
// non-nil empty slice is returned for an empty body so a zero-match list
// serialises as [] not null (get-many is consumed by Loop nodes).
func decodeList(resp *APIResponse) ([]interface{}, error) {
	if len(resp.Body) == 0 {
		return []interface{}{}, nil
	}
	out := []interface{}{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse WooCommerce response: %w", err)
	}
	return out, nil
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
// callers distinguish "unset" from "set to 0".
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Number() == nil {
		return 0, false
	}
	return int(*conn.Number()), true
}

// OptionalBool extracts a boolean input, defaulting to false when unset.
func OptionalBool(name string, inputs []*core.Connection) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return false
	}
	return *conn.Boolean()
}

// OptionalJSON parses an object/array-typed input into an arbitrary value. Used
// for the nested structures WooCommerce takes (line_items, billing, shipping,
// images, meta_data…) that have no flat-widget equivalent — the user supplies
// JSON. Returns (nil, nil) when absent/blank, (nil, err) on malformed JSON so
// the action can surface a clear message.
func OptionalJSON(name string, inputs []*core.Connection) (interface{}, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return nil, nil
	}
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
		// Already-parsed (from ${...} wiring).
		return conn.Value, nil
	}
}

// SetIfPresent adds an optional string field to a resource body only when the
// input was provided, so unset fields are omitted.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// SetIntIfPresent parses an optional string/integer input as an integer and adds
// it to the body when present. WooCommerce expects true JSON integers for id
// fields (customer_id, parent_id, product_id…); a quoted string is rejected.
// A non-numeric value is surfaced rather than silently dropped.
func SetIntIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) error {
	if n, ok := OptionalInt(inputName, inputs); ok {
		body[field] = n
		return nil
	}
	v := OptionalString(inputName, inputs)
	if v == "" {
		return nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fmt.Errorf("%s must be a whole number", inputName)
	}
	body[field] = n
	return nil
}

// SetBoolIfSet adds an optional boolean field when its input connection is
// present (checkbox touched), so the tri-state nil is preserved as "omit".
func SetBoolIfSet(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	conn := core.FindConnection(inputName, inputs)
	if conn != nil && conn.Boolean() != nil {
		body[field] = *conn.Boolean()
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

// SetIDRefsIfPresent maps a comma-separated list of ids (or a JSON array of ids)
// to WooCommerce's [{id: N}] reference shape, used by a product's categories and
// tags. Accepts "5,7" or "[5,7]". Skips blank/invalid entries. Omits the field
// entirely when nothing usable was supplied.
func SetIDRefsIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	raw := OptionalString(inputName, inputs)
	if raw == "" {
		return
	}
	raw = strings.Trim(raw, "[] ")
	refs := []interface{}{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.Trim(strings.TrimSpace(part), `"'`)
		if part == "" {
			continue
		}
		if n, err := strconv.Atoi(part); err == nil {
			refs = append(refs, map[string]interface{}{"id": n})
		}
	}
	if len(refs) > 0 {
		body[field] = refs
	}
}

// SetIntListIfPresent maps a comma-separated list of ids (or a JSON array of
// ids) to WooCommerce's plain [N] integer-array shape, used by a coupon's
// product_ids / product_categories (which — unlike a product's categories — are
// bare integer arrays, not [{id: N}] references). Accepts "5,7" or "[5,7]".
// Skips blank/invalid entries and omits the field when nothing usable remains.
func SetIntListIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	raw := OptionalString(inputName, inputs)
	if raw == "" {
		return
	}
	raw = strings.Trim(raw, "[] ")
	ids := []interface{}{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.Trim(strings.TrimSpace(part), `"'`)
		if part == "" {
			continue
		}
		if n, err := strconv.Atoi(part); err == nil {
			ids = append(ids, n)
		}
	}
	if len(ids) > 0 {
		body[field] = ids
	}
}

// MergeAdditionalFields overlays a raw JSON object input ("additional_fields")
// onto the resource body — the escape hatch for any WooCommerce field not exposed
// as a first-class input. Later keys win. Returns an error on malformed JSON or
// the wrong shape (an array/scalar).
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

// AddFilter sets a query param from an optional string input when non-empty.
func AddFilter(q url.Values, inputs []*core.Connection, param, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		q.Set(param, v)
	}
}

// ClampLimit bounds a requested per_page to WooCommerce's 1-100 range, falling
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

// ErrorResult is the standard soft-failure output map (returned with a nil error
// so the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ---------------------------------------------------------------------------
// Generic resource CRUD
// ---------------------------------------------------------------------------

// CreateResource POSTs a new resource. resourcePath is like "/orders". Unlike
// Shopify, WooCommerce takes and returns the bare object (no envelope).
func CreateResource(a Auth, resourcePath string, fields map[string]interface{}) (map[string]interface{}, error) {
	resp, err := ExecuteAPI(a, http.MethodPost, resourcePath, fields)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// GetResource GETs a single resource by path (e.g. "/orders/123"), applying
// optional query params.
func GetResource(a Auth, path string, q url.Values) (map[string]interface{}, error) {
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	resp, err := ExecuteAPI(a, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// UpdateResource PUTs changes to a single resource by path.
func UpdateResource(a Auth, path string, fields map[string]interface{}) (map[string]interface{}, error) {
	resp, err := ExecuteAPI(a, http.MethodPut, path, fields)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// DeleteResource DELETEs a single resource by path. force=true removes it
// permanently (required for customers, which have no trash; and the n8n default
// for all resources); force=false moves supported resources to the trash.
// WooCommerce returns the deleted object on success.
func DeleteResource(a Auth, path string, force bool) (map[string]interface{}, error) {
	q := url.Values{}
	if force {
		q.Set("force", "true")
	}
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	resp, err := ExecuteAPI(a, http.MethodDelete, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// ListResources fetches a collection. When returnAll is false a single page is
// fetched (page/per_page as supplied in q). When true it walks pages by
// incrementing "page" while the rel="next" Link header is present, up to the
// MaxAllPages cap. Returns the accumulated items, the X-WP-Total and
// X-WP-TotalPages counts from the first page, and the number of pages fetched.
func ListResources(a Auth, resourcePath string, q url.Values, returnAll bool) (items []interface{}, total, totalPages, pages int, err error) {
	items = []interface{}{}
	if q == nil {
		q = url.Values{}
	}
	if q.Get("per_page") == "" {
		q.Set("per_page", strconv.Itoa(DefaultPageLimit))
	}

	page := 1
	if p := q.Get("page"); p != "" {
		if n, e := strconv.Atoi(p); e == nil && n > 0 {
			page = n
		}
	}

	for {
		q.Set("page", strconv.Itoa(page))
		path := resourcePath + "?" + q.Encode()
		resp, e := ExecuteAPI(a, http.MethodGet, path, nil)
		if e != nil {
			return nil, 0, 0, pages, e
		}
		if e := CheckResponse(resp); e != nil {
			return nil, 0, 0, pages, e
		}
		arr, e := decodeList(resp)
		if e != nil {
			return nil, 0, 0, pages, e
		}
		items = append(items, arr...)
		pages++

		if pages == 1 {
			total = headerInt(resp.Headers, "X-WP-Total")
			totalPages = headerInt(resp.Headers, "X-WP-TotalPages")
		}

		hasNext := strings.Contains(resp.Headers.Get("Link"), `rel="next"`)
		if !returnAll || !hasNext || pages >= MaxAllPages {
			break
		}
		page++
	}
	return items, total, totalPages, pages, nil
}

// headerInt parses an integer response header, returning 0 when absent/invalid.
func headerInt(h http.Header, key string) int {
	if v := h.Get(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ResourceResult shapes a single-resource response (create/get/update/delete)
// into the standard action output. id is stringified from the object.
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          stringifyID(obj["id"]),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output: the
// results array, its length, the store-wide total and total-pages counts (from
// the WordPress headers), plus the mandatory summary/success/error triple.
func ListResult(items []interface{}, total, totalPages int, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"total":       total,
		"total_pages": totalPages,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// stringifyID renders WooCommerce's numeric IDs (which decode to float64 from
// JSON) as a clean integer string, leaving string IDs untouched.
func stringifyID(id interface{}) string {
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
