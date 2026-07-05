// Package wordpress holds the shared HTTP client, auth helpers, and generic
// resource CRUD used by every cms/wordpress/* action.
//
// The WordPress core REST API (v2) is uniform across resources — posts, pages,
// users, comments, categories and tags all share identical
// create/read/update/delete/list shapes under /wp-json/wp/v2/{resource}[/{id}].
// That regularity lets the CRUD live here once (CreateResource, GetResource,
// UpdateResource, DeleteResource, ListResources) so each action package stays
// thin — the same design as the sibling ecommerce/woocommerce package (which is
// no accident: WooCommerce is a WordPress plugin and shares its REST idioms).
//
// Three things shape this file:
//
//   - Responses are NOT enveloped: a single resource comes back as the bare
//     object, a collection as a bare JSON array.
//   - Pagination is page/per_page with X-WP-Total / X-WP-TotalPages count
//     headers and a rel="next" Link header.
//   - Updates use POST, not PUT — the WordPress REST API accepts POST to
//     /{resource}/{id} for partial updates (matching n8n's node).
//
// Auth is HTTP Basic — a WordPress username plus an Application Password (the
// modern per-app credential introduced in WP 5.6). The password is a
// ConnectionTypeSecret; the site URL and username are ConnectionTypeString. An
// optional "allow insecure SSL" flag routes requests through a client that skips
// TLS verification, for self-hosted sites with self-signed certificates
// (mirroring n8n's allowUnauthorizedCerts).
package wordpress

import (
	"bytes"
	"crypto/tls"
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
	// APIBasePath is the WordPress core REST API v2 prefix appended to the site
	// URL. Pinned to v2 (a single constant, not a per-action input) to keep the
	// UI clean for non-technical users.
	APIBasePath = "/wp-json/wp/v2"

	// maxResponseBody caps the response body to prevent memory exhaustion. Post
	// content can be large, so 8 MB (the woocommerce value).
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single WordPress call.
	requestTimeout = 30 * time.Second

	// MaxAllPages bounds a "return all" pagination loop so a site with a huge
	// archive can never spin unbounded requests. On hitting the cap the action
	// reports it and returns the next page number so the caller can resume.
	MaxAllPages = 100

	// DefaultPageLimit / MaxPageLimit bound WordPress's per_page (1-100).
	DefaultPageLimit = 50
	MaxPageLimit     = 100
)

// httpClient is shared across every WordPress action so TCP connections to the
// site are pooled and reused rather than re-dialled per call. insecureHTTPClient
// is the same but skips TLS verification, used only when the action opts in via
// "allow insecure SSL" — kept as a separate client so the secure default can
// never be weakened by a per-request tweak.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

var insecureHTTPClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // #nosec G402 — opt-in only, for self-signed self-hosted sites
	},
}

// AuthInputs is the shared credential shape. Action packages re-declare their
// own literal Inputs arrays (the manifest generator AST-parses those), but this
// documents the canonical set every action puts first.
var AuthInputs = []core.Connection{
	{
		Name:        "url",
		Type:        core.ConnectionTypeString,
		Label:       "Site URL",
		Placeholder: "https://your-site.com — your WordPress site root, not the /wp-json path",
		Required:    true,
	},
	{
		Name:        "username",
		Type:        core.ConnectionTypeString,
		Label:       "Username",
		Placeholder: "Your WordPress username",
		Required:    true,
	},
	{
		// Named app_password (not password) so a resource's own "password" field
		// — a post/page protection password, or a new user's account password —
		// can use the natural name without colliding with the auth credential.
		Name:        "app_password",
		Type:        core.ConnectionTypeSecret,
		Label:       "Application Password",
		Placeholder: "Users ▸ Profile ▸ Application Passwords (WP 5.6+)",
		Required:    true,
	},
	{
		Name:        "allow_insecure",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Allow Insecure SSL",
		Placeholder: "Skip TLS verification — only for self-hosted sites with a self-signed certificate",
	},
}

// Auth is the resolved connection: a normalised base URL (scheme + host [+ any
// subdirectory], no trailing slash, no /wp-json suffix), the username and
// Application Password, and the insecure-TLS opt-in.
type Auth struct {
	BaseURL  string
	Username string
	Password string
	Insecure bool
}

// APIResponse wraps the HTTP response for consistent handling. Headers is
// carried because pagination (rel="next") and total counts (X-WP-Total /
// X-WP-TotalPages) live in response headers.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ---------------------------------------------------------------------------
// Auth resolution
// ---------------------------------------------------------------------------

// GetAuth resolves the site URL, username and Application Password from the
// action's auth inputs. A missing part is a hard failure (zero Auth + real
// error) — there is nothing to attempt without them.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	rawURL, err := RequiredString("url", inputs)
	if err != nil {
		return Auth{}, err
	}
	base, err := NormaliseBaseURL(rawURL)
	if err != nil {
		return Auth{}, err
	}
	user, err := RequiredString("username", inputs)
	if err != nil {
		return Auth{}, err
	}
	pass, err := RequiredString("app_password", inputs)
	if err != nil {
		return Auth{}, err
	}
	return Auth{
		BaseURL:  base,
		Username: user,
		Password: pass,
		Insecure: OptionalBool("allow_insecure", inputs),
	}, nil
}

// NormaliseBaseURL reduces whatever the user pasted to a clean scheme+host[+path]
// base with no trailing slash and no REST-API suffix, defaulting to https. A
// subdirectory WordPress install (e.g. "site.com/blog") is preserved.
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
		return "", fmt.Errorf("url must be an http(s) URL, e.g. https://your-site.com")
	}
	if u.Host == "" {
		return "", fmt.Errorf("url must include a host, e.g. https://your-site.com")
	}
	path := strings.TrimRight(u.Path, "/")
	for _, suffix := range []string{APIBasePath, "/wp-json/wp/v1", "/wp-json"} {
		if strings.HasSuffix(path, suffix) {
			path = strings.TrimSuffix(path, suffix)
			break
		}
	}
	u.User = nil
	return u.Scheme + "://" + u.Host + path, nil
}

// apiBase assembles the REST API root for a site. It is a var (not inline) so
// SetBaseForTest can point every request at an httptest server.
var apiBase = func(a Auth) string {
	return strings.TrimRight(a.BaseURL, "/") + APIBasePath
}

// SetBaseForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores the real base. Test-only.
func SetBaseForTest(base string) func() {
	prev := apiBase
	apiBase = func(Auth) string { return strings.TrimRight(base, "/") }
	return func() { apiBase = prev }
}

func clientFor(a Auth) *http.Client {
	if a.Insecure {
		return insecureHTTPClient
	}
	return httpClient
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// ExecuteAPI performs a REST call to the site's WordPress API.
// method: GET, POST, DELETE (WordPress uses POST for updates, not PUT)
// path:   resource path including any query string (e.g. "/posts/12?force=true")
// body:   optional payload — marshalled to JSON for POST, ignored otherwise
func ExecuteAPI(a Auth, method, path string, body interface{}) (*APIResponse, error) {
	fullURL := apiBase(a) + path

	var bodyReader io.Reader
	if body != nil && method == http.MethodPost {
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

	req.SetBasicAuth(a.Username, a.Password)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := clientFor(a).Do(req)
	if err != nil {
		return nil, fmt.Errorf("WordPress API request failed: %s", redactAuth(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// redactAuth removes the Application Password from an error message. It travels
// in the Basic auth header (not the URL), so a leak is unlikely — but a proxy or
// wrapped error could echo it, so it is scrubbed defensively. A no-op when the
// value isn't present.
func redactAuth(a Auth, msg string) string {
	if a.Password != "" {
		msg = strings.ReplaceAll(msg, a.Password, "REDACTED")
	}
	return msg
}

// CheckResponse verifies a 2xx status, decoding WordPress's error envelope
// ({"code","message","data":{"status"}}). The message is the human-readable
// reason; code is appended when present so an unfamiliar error is greppable
// against the WordPress docs.
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
			return fmt.Errorf("WordPress API error (%d): %s [%s]", resp.StatusCode, env.Message, env.Code)
		}
		return fmt.Errorf("WordPress API error (%d): %s", resp.StatusCode, env.Message)
	}
	body := string(resp.Body)
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Errorf("WordPress API error (%d): %s", resp.StatusCode, body)
}

// decode unmarshals a successful single-resource body into a generic map.
func decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse WordPress response: %w", err)
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
		return nil, fmt.Errorf("failed to parse WordPress response: %w", err)
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

// OptionalInt extracts an integer input. The bool is false when absent.
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
// for nested structures (meta) that have no flat-widget equivalent. Returns
// (nil, nil) when absent/blank, (nil, err) on malformed JSON.
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
// it to the body when present. WordPress expects true JSON integers for id
// fields (author, parent, featured_media…). A non-numeric value is surfaced.
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

// SetIntListIfPresent maps a comma-separated list of ids (or a JSON array of
// ids) to WordPress's plain [N] integer-array shape, used by a post's
// categories / tags. Accepts "5,7" or "[5,7]". Skips blank/invalid entries and
// omits the field when nothing usable remains.
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

// SetStringListIfPresent maps a comma-separated list to a []string body field,
// used by a user's roles (WordPress takes roles as an array of role slugs, e.g.
// ["author","editor"]). Trims blanks; omits the field when nothing remains.
func SetStringListIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	raw := OptionalString(inputName, inputs)
	if raw == "" {
		return
	}
	vals := []interface{}{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			vals = append(vals, p)
		}
	}
	if len(vals) > 0 {
		body[field] = vals
	}
}

// MergeAdditionalFields overlays a raw JSON object input ("additional_fields")
// onto the resource body — the escape hatch for any WordPress field not exposed
// as a first-class input. Returns an error on malformed JSON or the wrong shape.
//
// It is called LAST in every action's body assembly, so a key here OVERRIDES the
// same key set by a first-class input. This "power-user last word" precedence is
// deliberate and matches the WooCommerce / Cal.com / Acuity nodes.
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

// ClampLimit bounds a requested per_page to WordPress's 1-100 range, falling
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

// CreateResource POSTs a new resource. WordPress takes and returns the bare
// object (no envelope).
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

// GetResource GETs a single resource by path (e.g. "/posts/12"), applying
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

// UpdateResource POSTs changes to a single resource by path. WordPress uses POST
// (not PUT) to update; unspecified fields are left unchanged.
func UpdateResource(a Auth, path string, fields map[string]interface{}) (map[string]interface{}, error) {
	resp, err := ExecuteAPI(a, http.MethodPost, path, fields)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// DeleteResource DELETEs a single resource by path. force=true removes it
// permanently (required for users/terms, which have no trash); force=false
// moves supported resources (posts/pages/comments) to the trash. extra carries
// resource-specific delete params (e.g. a user's reassign target).
func DeleteResource(a Auth, path string, force bool, extra url.Values) (map[string]interface{}, error) {
	q := url.Values{}
	for k, vs := range extra {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
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

// ListResult shapes a collection response into the standard list output.
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

// stringifyID renders WordPress's numeric IDs (which decode to float64 from
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
