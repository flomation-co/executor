// Package shopify holds the shared HTTP client, auth helpers, and generic
// resource CRUD used by every ecommerce/shopify/* action.
//
// Shopify's Admin REST API is uniform across resources — orders and products
// share identical create/read/update/delete/list shapes under
// /admin/api/{version}/{resource}(.json). That regularity lets the CRUD live
// here once (CreateResource, GetResource, UpdateResource, DeleteResource,
// ListResources) so each action package stays thin: read its inputs, call one
// helper, shape the result.
//
// Auth is a Shopify Admin API access token carried in the custom
// X-Shopify-Access-Token header (NOT Authorization: Bearer), plus a shop
// subdomain that forms the base URL — the modern custom-app credential.
// The token is a ConnectionTypeSecret; the shop subdomain a ConnectionTypeString.
package shopify

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// APIVersion pins the Shopify Admin API version. Shopify versions are
	// date-stamped and supported for ~12 months; bump this const on upgrade
	// (it is deliberately a single constant rather than a per-action input,
	// mirroring gitlab.APIPath — a version selector on every node would just
	// clutter the UI for non-technical users).
	APIVersion = "2025-01"

	// maxResponseBody caps the response body to prevent memory exhaustion.
	// List pages of orders/products can be large, so 8 MB (the airtable
	// value) rather than the 1 MB used by single-record integrations.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single Shopify call.
	requestTimeout = 30 * time.Second

	// MaxAllPages bounds a "return all" pagination loop so a store with a
	// huge catalogue can never spin unbounded requests. On hitting the cap
	// the action returns the outstanding page cursor so the caller resumes.
	MaxAllPages = 100

	// DefaultPageLimit / MaxPageLimit bound Shopify's per-page limit (1-250).
	DefaultPageLimit = 50
	MaxPageLimit     = 250
)

// httpClient is shared across every Shopify action so TCP connections to the
// shop's Admin API are pooled and reused rather than re-dialled per call (a
// flow run — or a return-all loop — can fire many requests). Matches the
// connection-reuse pattern used by the HubSpot and Airtable integrations.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AuthInputs is the shared credential shape. Action packages re-declare their
// own literal Inputs arrays (the manifest generator AST-parses those), but
// this documents the canonical pair every action puts first.
var AuthInputs = []core.Connection{
	{
		Name:        "shop",
		Type:        core.ConnectionTypeString,
		Label:       "Shop Subdomain",
		Placeholder: "my-store (from my-store.myshopify.com)",
		Required:    true,
	},
	{
		Name:        "access_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Admin API Access Token",
		Placeholder: "shpat_... — or use Client ID + Secret below",
	},
	{
		Name:        "client_id",
		Type:        core.ConnectionTypeString,
		Label:       "Client ID",
		Placeholder: "Dev Dashboard app Client ID (if not using a token)",
	},
	{
		Name:        "client_secret",
		Type:        core.ConnectionTypeSecret,
		Label:       "Client Secret",
		Placeholder: "shpss_... — minted into a 24h token automatically",
	},
}

// APIResponse wraps the HTTP response for consistent handling. Headers is
// carried because Shopify's list pagination cursor lives in the Link header.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// validShopHandle matches a Shopify store handle: letters, numbers and
// hyphens only (Shopify handles are lowercase, but we accept either case and
// reject anything with host-significant characters). GetAuth enforces this so
// a crafted shop value can never redirect the access token off myshopify.com.
var validShopHandle = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]*$`)

// NormaliseShop reduces whatever the user pasted to the bare store handle.
// Accepts "my-store", "my-store.myshopify.com", a full "*.myshopify.com" URL,
// or the new-style "admin.shopify.com/store/my-store" admin URL, and returns
// "my-store". The result is charset-validated by GetAuth before use.
func NormaliseShop(shop string) string {
	shop = strings.TrimSpace(shop)
	shop = strings.TrimPrefix(shop, "https://")
	shop = strings.TrimPrefix(shop, "http://")
	shop = strings.TrimRight(shop, "/")
	// New-style admin URL: admin.shopify.com/store/<handle>.
	if i := strings.Index(shop, "admin.shopify.com/store/"); i >= 0 {
		rest := shop[i+len("admin.shopify.com/store/"):]
		if j := strings.IndexAny(rest, "/?#"); j >= 0 {
			rest = rest[:j]
		}
		return rest
	}
	shop = strings.TrimSuffix(shop, ".myshopify.com")
	// Drop anything from the first host-significant character onward so a
	// pasted path/port/userinfo can't leak into the assembled host.
	if i := strings.IndexAny(shop, "/.?#:@"); i >= 0 {
		shop = shop[:i]
	}
	return shop
}

// hostForShop returns the scheme+host for a shop's Admin API. It is a var
// rather than inline so tests can point every request at an httptest server
// (the same seam idiom as the openrouter action's apiURL var).
var hostForShop = func(shop string) string {
	return fmt.Sprintf("https://%s.myshopify.com", shop)
}

// BuildURL assembles a full Admin API URL for a resource path (which must
// start with "/", e.g. "/orders.json").
func BuildURL(shop, path string) string {
	return hostForShop(shop) + "/admin/api/" + APIVersion + path
}

// SetHostForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores the real host. It lets action
// packages in sibling directories exercise Execute end-to-end without hitting
// Shopify. Test-only.
func SetHostForTest(base string) func() {
	prev := hostForShop
	hostForShop = func(string) string { return base }
	return func() { hostForShop = prev }
}

// ExecuteAPI performs a REST call to the shop's Admin API.
// method: GET, POST, PUT, DELETE
// path:   resource path including any query string (e.g. "/orders/123.json?fields=id")
// body:   optional payload — marshalled to JSON for POST/PUT, ignored otherwise
func ExecuteAPI(shop, token, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := BuildURL(shop, path)

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
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("X-Shopify-Access-Token", token)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Shopify API request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// CheckResponse verifies a 2xx status, decoding Shopify's error envelope. The
// "errors" field may be a plain string ("Not Found") or an object keyed by
// field ({"title": ["can't be blank"]}), so it is modelled as interface{} and
// re-encoded when it isn't a string. 429 is surfaced with its Retry-After so
// the caller understands it hit Shopify's rate limit (Shopify enforces a
// leaky-bucket the way n8n's node does not guard against).
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if resp.StatusCode == http.StatusTooManyRequests {
		retry := resp.Headers.Get("Retry-After")
		if retry != "" {
			return fmt.Errorf("Shopify API rate limit exceeded (429); retry after %ss", retry)
		}
		return fmt.Errorf("Shopify API rate limit exceeded (429)")
	}

	var env struct {
		Errors interface{} `json:"errors"`
		Error  string      `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &env); err == nil {
		if msg := formatErrors(env.Errors); msg != "" {
			return fmt.Errorf("Shopify API error (%d): %s", resp.StatusCode, msg)
		}
		if env.Error != "" {
			return fmt.Errorf("Shopify API error (%d): %s", resp.StatusCode, env.Error)
		}
	}
	return fmt.Errorf("Shopify API error (%d): %s", resp.StatusCode, string(resp.Body))
}

func formatErrors(errs interface{}) string {
	switch v := errs.(type) {
	case nil:
		return ""
	case string:
		return v
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return ""
		}
		return string(b)
	}
}

// decode unmarshals a successful response body into a generic map. An empty
// body (e.g. a delete's 200 with no content) yields an empty map.
func decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Shopify response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// GetAuth resolves the shop handle and an access token from the action's
// auth inputs. Two modes are supported, in priority order:
//
//  1. Access Token — a shpat_/shpca_ token supplied directly (access_token).
//  2. Client Credentials — a Client ID + Client Secret (client_id +
//     client_secret) that are exchanged for a short-lived token via Shopify's
//     client credentials grant, cached until near expiry so a flow that fires
//     many Shopify actions mints the token once. This is the modern
//     Dev-Dashboard credential (Shopify no longer issues permanent tokens).
//
// A missing shop, or neither credential form, is a hard failure (nil result +
// real error) rather than a soft error output.
func GetAuth(inputs []*core.Connection) (shop, token string, err error) {
	shopRaw, err := RequiredString("shop", inputs)
	if err != nil {
		return "", "", err
	}
	shop = NormaliseShop(shopRaw)
	if !validShopHandle.MatchString(shop) {
		return "", "", fmt.Errorf("shop must be your store handle (letters, numbers, hyphens) — e.g. my-store from my-store.myshopify.com")
	}

	// A directly-supplied token wins — no round-trip needed.
	if tok := OptionalString("access_token", inputs); tok != "" {
		return shop, tok, nil
	}

	// Otherwise mint one from Client ID + Secret.
	clientID := OptionalString("client_id", inputs)
	clientSecret := OptionalString("client_secret", inputs)
	if clientID != "" && clientSecret != "" {
		tok, err := getOrMintToken(shop, clientID, clientSecret)
		if err != nil {
			return "", "", err
		}
		return shop, tok, nil
	}

	return "", "", fmt.Errorf("authentication required: provide an Access Token, or a Client ID and Client Secret to mint one")
}

// cachedToken is a minted access token and the time it should be refreshed by.
type cachedToken struct {
	token     string
	expiresAt time.Time
}

// tokenCache holds client-credentials tokens keyed by shop+client_id so a flow
// firing several Shopify actions mints the 24h token once rather than per call.
// Same mutex+expiry idiom as the api's option-proxy caches.
var tokenCache = struct {
	mu sync.Mutex
	m  map[string]cachedToken
}{m: map[string]cachedToken{}}

func getOrMintToken(shop, clientID, clientSecret string) (string, error) {
	key := shop + "|" + clientID
	tokenCache.mu.Lock()
	if c, ok := tokenCache.m[key]; ok && time.Now().Before(c.expiresAt) {
		tokenCache.mu.Unlock()
		return c.token, nil
	}
	tokenCache.mu.Unlock()

	// Mint outside the lock so a slow token endpoint doesn't block other
	// shops; a concurrent double-mint is harmless (same credentials).
	tok, expiresIn, err := MintAccessToken(shop, clientID, clientSecret)
	if err != nil {
		return "", err
	}
	ttl := time.Duration(expiresIn) * time.Second
	if ttl <= 0 {
		ttl = 23 * time.Hour
	}
	if ttl > 5*time.Minute {
		ttl -= 5 * time.Minute // refresh a little before actual expiry
	}
	tokenCache.mu.Lock()
	tokenCache.m[key] = cachedToken{token: tok, expiresAt: time.Now().Add(ttl)}
	tokenCache.mu.Unlock()
	return tok, nil
}

// MintAccessToken exchanges a Client ID + Secret for an access token via
// Shopify's client credentials grant. The app must be installed on the store
// and share its organisation. Returns the token and its lifetime in seconds.
func MintAccessToken(shop, clientID, clientSecret string) (string, int, error) {
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", clientID)
	form.Set("client_secret", clientSecret)

	req, err := http.NewRequest(http.MethodPost, hostForShop(shop)+"/admin/oauth/access_token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, fmt.Errorf("failed to create token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("Shopify token request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))

	if resp.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("Shopify token request failed (%d): %s — check the Client ID/Secret, that the app is installed on the store, and that the app and store share an organisation", resp.StatusCode, extractTokenError(body))
	}

	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", 0, fmt.Errorf("failed to parse Shopify token response: %w", err)
	}
	if out.AccessToken == "" {
		return "", 0, fmt.Errorf("Shopify token response contained no access_token")
	}
	return out.AccessToken, out.ExpiresIn, nil
}

// extractTokenError pulls a readable message from a token-endpoint error, which
// may be JSON ({error, error_description}) or an HTML page whose <title> holds
// the OAuth error (e.g. "400 - Oauth error app_not_installed").
func extractTokenError(body []byte) string {
	var j struct {
		Error     string `json:"error"`
		ErrorDesc string `json:"error_description"`
	}
	if json.Unmarshal(body, &j) == nil && (j.Error != "" || j.ErrorDesc != "") {
		if j.ErrorDesc != "" {
			return j.ErrorDesc
		}
		return j.Error
	}
	s := string(body)
	if i := strings.Index(s, "<title>"); i >= 0 {
		if k := strings.Index(s[i:], "</title>"); k >= 0 {
			return strings.TrimSpace(s[i+len("<title>") : i+k])
		}
	}
	if len(s) > 200 {
		s = s[:200]
	}
	return strings.TrimSpace(s)
}

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

// OptionalJSON parses an object/array-typed input into an arbitrary value.
// Used for the nested structures Shopify takes (line_items, addresses,
// images, variants) that have no flat-widget equivalent — the user supplies
// JSON. Returns (nil, nil) when the input is absent/blank, (nil, err) on
// malformed JSON so the action can surface a clear message.
func OptionalJSON(name string, inputs []*core.Connection) (interface{}, error) {
	conn := core.FindConnection(name, inputs)
	if conn == nil {
		return nil, nil
	}
	// Object/array inputs may arrive already-parsed (from ${...} wiring) or
	// as a JSON string typed into the editor.
	if conn.Value != nil {
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
	return nil, nil
}

// SetIfPresent adds an optional string field to a resource body only when the
// input was provided, so unset fields are omitted (Shopify treats an omitted
// field and an empty string differently for some fields).
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
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

// MergeAdditionalFields overlays a raw JSON object input ("additional_fields")
// onto the resource body, the escape hatch for any Shopify field not exposed
// as a first-class input. Later keys win. Returns an error on malformed JSON.
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
		// Valid JSON but the wrong shape (an array or scalar) — surface it
		// rather than silently dropping the fields, matching airtable's
		// mergeJSONObject convention.
		return fmt.Errorf(`additional_fields must be a JSON object, e.g. {"key":"value"}`)
	}
	for k, val := range obj {
		body[k] = val
	}
	return nil
}

// BuildDefaultVariant assembles a single default variant from the first-class
// price/sku inputs so a non-technical user can set a product's price without
// hand-writing the Variants JSON. Returns nil when neither is set (Shopify
// then auto-creates a $0.00 default variant). Callers use it only when no
// explicit Variants JSON was supplied.
func BuildDefaultVariant(inputs []*core.Connection) []interface{} {
	variant := map[string]interface{}{}
	if v := OptionalString("price", inputs); v != "" {
		variant["price"] = v
	}
	if v := OptionalString("sku", inputs); v != "" {
		variant["sku"] = v
	}
	if len(variant) == 0 {
		return nil
	}
	return []interface{}{variant}
}

// ErrorResult is the standard soft-failure output map (returned with a nil
// error so the engine records it as data on the error port).
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

// CreateResource POSTs a new resource. resourcePath is like "/orders.json";
// resourceKey ("order"/"product") wraps the body and unwraps the response.
func CreateResource(shop, token, resourcePath, resourceKey string, fields map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{resourceKey: fields}
	resp, err := ExecuteAPI(shop, token, http.MethodPost, resourcePath, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// GetResource GETs a single resource by path (e.g. "/orders/123.json"),
// applying optional query params (e.g. fields).
func GetResource(shop, token, path string, q url.Values) (map[string]interface{}, error) {
	if enc := q.Encode(); enc != "" {
		path += "?" + enc
	}
	resp, err := ExecuteAPI(shop, token, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// UpdateResource PUTs changes to a single resource by path.
func UpdateResource(shop, token, path, resourceKey string, fields map[string]interface{}) (map[string]interface{}, error) {
	payload := map[string]interface{}{resourceKey: fields}
	resp, err := ExecuteAPI(shop, token, http.MethodPut, path, payload)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// DeleteResource DELETEs a single resource by path. Shopify returns 200 with
// an empty JSON object on success.
func DeleteResource(shop, token, path string) error {
	resp, err := ExecuteAPI(shop, token, http.MethodDelete, path, nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// ListResources fetches a collection. propertyName is the plural envelope key
// ("orders"/"products"). When returnAll is false a single page is fetched and
// the next-page cursor (if any) is returned so the caller can resume manually.
// When true it follows the Link-header "next" cursor until exhausted or the
// MaxAllPages cap, honouring Shopify's page_info restriction (only limit and
// fields are permitted alongside page_info). Returns the accumulated items,
// the outstanding next page_info (empty when fully drained), the last raw
// response, and the number of pages fetched.
func ListResources(shop, token, resourcePath, propertyName string, q url.Values, returnAll bool) (items []interface{}, nextPageInfo string, lastRaw map[string]interface{}, pages int, err error) {
	// Non-nil so a zero-match list serialises as [] not null — get-many is
	// consumed by Loop nodes that iterate the array.
	items = []interface{}{}
	if q == nil {
		q = url.Values{}
	}
	// Preserve limit/fields for building page_info-only follow-up queries.
	limit := q.Get("limit")
	fields := q.Get("fields")

	for {
		path := resourcePath
		if enc := q.Encode(); enc != "" {
			path += "?" + enc
		}
		resp, e := ExecuteAPI(shop, token, http.MethodGet, path, nil)
		if e != nil {
			return nil, "", nil, pages, e
		}
		if e := CheckResponse(resp); e != nil {
			return nil, "", nil, pages, e
		}
		raw, e := decode(resp)
		if e != nil {
			return nil, "", nil, pages, e
		}
		lastRaw = raw
		pages++
		if arr, ok := raw[propertyName].([]interface{}); ok {
			items = append(items, arr...)
		}

		nextPageInfo = parseNextPageInfo(resp.Headers.Get("Link"))
		if !returnAll || nextPageInfo == "" || pages >= MaxAllPages {
			break
		}
		// With page_info present Shopify permits only limit and fields.
		q = url.Values{}
		q.Set("page_info", nextPageInfo)
		if limit != "" {
			q.Set("limit", limit)
		}
		if fields != "" {
			q.Set("fields", fields)
		}
	}
	return items, nextPageInfo, lastRaw, pages, nil
}

// parseNextPageInfo extracts the page_info cursor from the rel="next" entry of
// a Shopify Link response header (RFC 5988). Returns "" when there is no next
// page. Example header:
//
//	<https://x.myshopify.com/admin/api/2025-01/orders.json?limit=50&page_info=abc>; rel="next"
func parseNextPageInfo(link string) string {
	if link == "" {
		return ""
	}
	for _, part := range strings.Split(link, ",") {
		part = strings.TrimSpace(part)
		if !strings.Contains(part, `rel="next"`) {
			continue
		}
		start := strings.Index(part, "<")
		end := strings.Index(part, ">")
		if start < 0 || end <= start {
			continue
		}
		u, err := url.Parse(part[start+1 : end])
		if err != nil {
			return ""
		}
		return u.Query().Get("page_info")
	}
	return ""
}

// ClampLimit bounds a requested per-page limit to Shopify's 1-250 range,
// falling back to DefaultPageLimit when unset.
func ClampLimit(limit int, set bool) int {
	if !set || limit <= 0 {
		return DefaultPageLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit
	}
	return limit
}

// AddFilter sets a query param from an optional string input when non-empty.
func AddFilter(q url.Values, inputs []*core.Connection, param, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		q.Set(param, v)
	}
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ResourceResult shapes a single-resource response (create/get/update) into
// the standard action output. resourceKey unwraps Shopify's {"order": {...}}
// envelope; id is stringified from the unwrapped object.
func ResourceResult(resp map[string]interface{}, resourceKey, summary string) map[string]interface{} {
	obj, _ := resp[resourceKey].(map[string]interface{})
	if obj == nil {
		obj = resp
	}
	return map[string]interface{}{
		"id":          stringifyID(obj["id"]),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output:
// results array, count, the outstanding next page_info cursor, the raw last
// page, plus the mandatory summary/success/error triple.
func ListResult(items []interface{}, nextPageInfo string, lastRaw map[string]interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":        items,
		"count":          len(items),
		"next_page_info": nextPageInfo,
		"result":         lastRaw,
		"tool_result":    summary,
		"success":        true,
		"error":          "",
	}
}

// stringifyID renders Shopify's numeric IDs (which decode to float64 from
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
