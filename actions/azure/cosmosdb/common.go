// Package cosmosdb holds the shared master-key signer, Entra header builder,
// HTTP plumbing, partition-key discovery and feed pagination used by every
// azure/cosmosdb/* action.
//
// The Cosmos DB (NoSQL) REST API has four sharp edges this file absorbs so the
// actions stay thin:
//
//   - Master-key auth is a per-request HMAC-SHA256 over
//     "{verb}\n{resourceType}\n{resourceId}\n{date}\n\n" (verb, type and date
//     lowercased; resourceId case-SENSITIVE), and the authorization header
//     value must be URL-encoded. resourceType/resourceId are passed EXPLICITLY
//     by each call site — we build the URLs, so we never re-derive the resource
//     link by parsing them back out (the n8n approach, which cannot sign
//     /offers at all).
//   - Every collection response is enveloped ({"Documents":[...]},
//     {"DocumentCollections":[...]}, {"Databases":[...]}, {"Offers":[...]})
//     and paginated via opaque x-ms-continuation request/response headers.
//   - Point operations on partitioned containers need the partition-key value
//     in an x-ms-documentdb-partitionkey header, formatted as a JSON array
//     literal. The container's partition-key PATH must be discovered with a
//     GET on the container — done once per execution and cached.
//   - Errors are {code, message} where message often wraps a nested JSON
//     payload ("Message: {\"Errors\":[...]}") that carries the real reason.
//
// The database is a per-action input, never pinned in the credential — one
// connection reaches every database in the account (n8n bakes a single
// database into its credential; we deliberately do not).
package cosmosdb

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	core "flomation.app/automate/executor"
	azure "flomation.app/automate/executor/actions/azure"
)

const (
	// APIVersion is pinned on every request via x-ms-version. 2018-12-31 is the
	// newest GA REST version; partial-document PATCH and autoscale offers are
	// both served under it.
	APIVersion = "2018-12-31"

	// maxResponseBody caps response bodies to prevent memory exhaustion.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single Cosmos call.
	requestTimeout = 60 * time.Second

	// DefaultPageLimit / MaxPageLimit bound x-ms-max-item-count. Cosmos accepts
	// up to 1000 items per page.
	DefaultPageLimit = 50
	MaxPageLimit     = 1000

	// MaxAllPages bounds a "return all" continuation loop so a huge container
	// can never spin unbounded requests (at the 50-item default page size this
	// is 10k items per run).
	MaxAllPages = 200
)

// AuthMethod values for the auth_method dropdown. The empty string (a fresh,
// untouched dropdown) means master key.
const (
	AuthMethodMasterKey = "master_key"
	AuthMethodEntra     = "entra"
)

// httpClient is shared across every Cosmos action so TLS connections to the
// account are pooled and reused rather than re-dialled per call.
// insecureHTTPClient is the same but skips TLS verification — the Cosmos DB
// emulator terminates TLS with a self-signed certificate, so local development
// is impossible without it. Kept as a separate client so the secure default
// can never be weakened by a per-request tweak.
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
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // #nosec G402 — opt-in only, for the emulator's self-signed certificate
	},
}

// AuthInputs is the canonical credential block. Action packages re-declare
// their own literal Inputs arrays (the manifest generator AST-parses those
// literals, so it cannot see through a shared variable), but every action puts
// these eight first, in this order — cosmosdb_inputs_drift_test.go enforces
// it. The names are reserved: a resource field must never reuse one, because
// core.FindConnection returns the FIRST name match and the credential block is
// declared first.
var AuthInputs = []core.Connection{
	{
		Name:        "account_name",
		Type:        core.ConnectionTypeString,
		Label:       "Account Name",
		Placeholder: "mycosmosaccount",
		Required:    true,
	},
	{
		Name:  "auth_method",
		Type:  core.ConnectionTypeString,
		Label: "Authentication",
		Options: []core.ConnectionOption{
			{Name: "Master Key", Value: AuthMethodMasterKey},
			{Name: "Microsoft Entra (service principal)", Value: AuthMethodEntra},
		},
	},
	{
		Name:        "master_key",
		Type:        core.ConnectionTypeSecret,
		Label:       "Master Key",
		Placeholder: "Primary or secondary key (base64) — Azure Portal ▸ your account ▸ Keys",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{"", AuthMethodMasterKey}},
	},
	{
		Name:        "azure_tenant_id",
		Type:        core.ConnectionTypeString,
		Label:       "Tenant ID",
		Placeholder: "Directory (tenant) ID of the service principal",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthMethodEntra}},
	},
	{
		Name:        "azure_client_id",
		Type:        core.ConnectionTypeString,
		Label:       "Client ID",
		Placeholder: "Application (client) ID of the service principal",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthMethodEntra}},
	},
	{
		Name:        "azure_client_secret",
		Type:        core.ConnectionTypeSecret,
		Label:       "Client Secret",
		Placeholder: "The service principal's client secret",
		Visible:     &core.VisibleWhen{Field: "auth_method", Values: []string{AuthMethodEntra}},
	},
	{
		Name:        "endpoint",
		Type:        core.ConnectionTypeString,
		Label:       "Custom Endpoint",
		Placeholder: "https://localhost:8081 for the emulator — leave blank for https://{account}.documents.azure.com",
	},
	{
		Name:        "allow_insecure",
		Type:        core.ConnectionTypeBoolean,
		Label:       "Allow Insecure TLS",
		Placeholder: "Skip TLS verification — required for the Cosmos DB emulator's self-signed certificate",
	},
}

// Auth is the resolved credential: the account base URL, the chosen auth
// method and its material, and the insecure-TLS opt-in.
type Auth struct {
	AccountName  string
	BaseURL      string // scheme://host[:port], no trailing slash
	Method       string // AuthMethodMasterKey or AuthMethodEntra
	MasterKey    string
	TenantID     string
	ClientID     string
	ClientSecret string
	Insecure     bool
}

// APIResponse wraps the HTTP response. Headers are carried because pagination
// (x-ms-continuation) and RU accounting (x-ms-request-charge) live there.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ---------------------------------------------------------------------------
// Auth resolution
// ---------------------------------------------------------------------------

// GetAuth resolves the credential block. A missing or malformed part is a hard
// failure (zero Auth + real error) — there is nothing to attempt without it.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	account, err := RequiredString("account_name", inputs)
	if err != nil {
		return Auth{}, err
	}
	// The account name is interpolated into a hostname; restrict it to the
	// charset Azure itself allows so a crafted value cannot rewrite the URL.
	for _, r := range account {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') && (r < '0' || r > '9') && r != '-' {
			return Auth{}, fmt.Errorf("account_name %q contains invalid characters — letters, digits and hyphens only", account)
		}
	}

	a := Auth{
		AccountName: account,
		BaseURL:     "https://" + account + ".documents.azure.com",
		Insecure:    OptionalBool("allow_insecure", inputs),
	}

	if raw := OptionalString("endpoint", inputs); raw != "" {
		u, err := url.Parse(raw)
		if err != nil {
			return Auth{}, fmt.Errorf("endpoint is not a valid URL: %w", err)
		}
		if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return Auth{}, fmt.Errorf("endpoint must be an http(s) URL, e.g. https://localhost:8081")
		}
		a.BaseURL = u.Scheme + "://" + u.Host + strings.TrimRight(u.Path, "/")
	}

	switch method := OptionalString("auth_method", inputs); method {
	case "", AuthMethodMasterKey:
		a.Method = AuthMethodMasterKey
		key, err := RequiredString("master_key", inputs)
		if err != nil {
			return Auth{}, fmt.Errorf("master_key is required when Authentication is Master Key")
		}
		a.MasterKey = key
	case AuthMethodEntra:
		a.Method = AuthMethodEntra
		for name, dst := range map[string]*string{
			"azure_tenant_id":     &a.TenantID,
			"azure_client_id":     &a.ClientID,
			"azure_client_secret": &a.ClientSecret,
		} {
			v, err := RequiredString(name, inputs)
			if err != nil {
				return Auth{}, fmt.Errorf("%s is required when Authentication is Microsoft Entra", name)
			}
			*dst = v
		}
	default:
		return Auth{}, fmt.Errorf("auth_method %q is not supported — use master_key or entra", method)
	}

	return a, nil
}

// EntraScope derives the AAD token scope from the account host. Cosmos RBAC
// tokens are scoped to the account endpoint itself (not a generic resource),
// so a custom/sovereign endpoint changes the scope too.
func EntraScope(a Auth) string {
	host := a.AccountName + ".documents.azure.com"
	if u, err := url.Parse(a.BaseURL); err == nil && u.Hostname() != "" {
		host = u.Hostname()
	}
	return "https://" + host + "/.default"
}

// Context returns the flow's Go context, tolerating a nil flow (as in tests).
func Context(flow *core.Flow) context.Context {
	if flow != nil {
		return flow.GoContext()
	}
	return context.Background()
}

func clientFor(a Auth) *http.Client {
	if a.Insecure {
		return insecureHTTPClient
	}
	return httpClient
}

// redact scrubs credentials from an error message before it reaches an output.
func redact(a Auth, msg string) string {
	msg = azure.RedactSecret(msg, a.MasterKey)
	return azure.RedactSecret(msg, a.ClientSecret)
}

// ---------------------------------------------------------------------------
// Signing
// ---------------------------------------------------------------------------

// MasterKeyAuthHeader computes the master-key authorization header value:
// HMAC-SHA256 over "{verb}\n{resourceType}\n{resourceId}\n{date}\n\n" with
// verb, resourceType and date lowercased (resourceId is case-SENSITIVE — the
// one exception is a single offer, whose _rid the CALLER must lowercase), key
// = the base64-decoded master key, and the whole "type=master&ver=1.0&sig="
// value URL-encoded, as the service requires.
func MasterKeyAuthHeader(verb, resourceType, resourceID, date, masterKey string) (string, error) {
	key, err := base64.StdEncoding.DecodeString(masterKey)
	if err != nil {
		// Never echo the key material itself.
		return "", fmt.Errorf("master key is not valid base64 — copy it from Azure Portal ▸ Keys")
	}
	payload := strings.ToLower(verb) + "\n" +
		strings.ToLower(resourceType) + "\n" +
		resourceID + "\n" +
		strings.ToLower(date) + "\n" +
		"\n"
	mac := hmac.New(sha256.New, key)
	mac.Write([]byte(payload))
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return url.QueryEscape("type=master&ver=1.0&sig=" + sig), nil
}

// AADAuthHeader wraps an Entra bearer token in the Cosmos authorization
// scheme. Same URL-encoding requirement as the master-key form.
func AADAuthHeader(token string) string {
	return url.QueryEscape("type=aad&ver=1.0&sig=" + token)
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// DoRequest performs one signed Cosmos call. resourceType/resourceID are the
// signing identity of the target resource and are supplied explicitly by the
// call site (see the path/RID builders below); they are unused under Entra
// auth but always passed so a call site cannot compile without thinking about
// them. headers carries per-operation extras (partition key, upsert, query
// markers, If-Match, Content-Type overrides).
func DoRequest(flow *core.Flow, a Auth, method, path, resourceType, resourceID string, headers map[string]string, body []byte) (*APIResponse, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(Context(flow), method, a.BaseURL+path, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %s", redact(a, err.Error()))
	}

	date := time.Now().UTC().Format(http.TimeFormat)
	req.Header.Set("x-ms-date", date)
	req.Header.Set("x-ms-version", APIVersion)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	switch a.Method {
	case AuthMethodEntra:
		token, err := azure.ClientCredentialsToken(Context(flow), clientFor(a), a.TenantID, a.ClientID, a.ClientSecret, EntraScope(a))
		if err != nil {
			return nil, err // already redacted by the azure package
		}
		req.Header.Set("Authorization", AADAuthHeader(token))
	default:
		auth, err := MasterKeyAuthHeader(method, resourceType, resourceID, date, a.MasterKey)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Authorization", auth)
	}

	resp, err := clientFor(a).Do(req)
	if err != nil {
		return nil, fmt.Errorf("Cosmos DB request failed: %s", redact(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %s", redact(a, err.Error()))
	}
	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// CheckResponse verifies a 2xx status, decoding Cosmos's {code, message} error
// envelope. The message frequently wraps the real reason in nested JSON
// ("Message: {\"Errors\":[...]}\r\nActivityId: ...") — unwrapped so the
// operator sees "Resource with specified id already exists", not a wall of
// diagnostics.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	var env struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(resp.Body, &env); err == nil && env.Message != "" {
		msg := unwrapNestedErrors(env.Message)
		if env.Code != "" {
			return fmt.Errorf("Cosmos DB error (%d %s): %s", resp.StatusCode, env.Code, msg)
		}
		return fmt.Errorf("Cosmos DB error (%d): %s", resp.StatusCode, msg)
	}
	body := string(resp.Body)
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Errorf("Cosmos DB error (%d): %s", resp.StatusCode, body)
}

// unwrapNestedErrors extracts the Errors array from a message of the form
// `Message: {"Errors":["..."]}` (trailing ActivityId/Request URI diagnostics
// dropped). Returns the original message when the pattern is absent.
func unwrapNestedErrors(msg string) string {
	idx := strings.Index(msg, "Message: {")
	if idx < 0 {
		return strings.TrimSpace(msg)
	}
	jsonPart := msg[idx+len("Message: "):]
	if end := strings.IndexAny(jsonPart, "\r\n"); end >= 0 {
		jsonPart = jsonPart[:end]
	}
	var nested struct {
		Errors []string `json:"Errors"`
	}
	if err := json.Unmarshal([]byte(jsonPart), &nested); err == nil && len(nested.Errors) > 0 {
		return strings.Join(nested.Errors, "; ")
	}
	return strings.TrimSpace(msg)
}

// DecodeObject unmarshals a successful body into a generic map. Empty bodies
// (204 deletes) decode to an empty map.
func DecodeObject(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Cosmos DB response: %w", err)
	}
	return out, nil
}

// RequestCharge returns the RU cost of a single response, verbatim from the
// x-ms-request-charge header ("" when absent).
func RequestCharge(resp *APIResponse) string {
	return resp.Headers.Get("x-ms-request-charge")
}

// ---------------------------------------------------------------------------
// Resource paths and signing RIDs
// ---------------------------------------------------------------------------
//
// Paths are what goes on the wire (each id segment-encoded — Cosmos ids may
// contain spaces); RIDs are the signing resourceId, which the service computes
// against the RAW resource link, so they are built from the unescaped ids.

func DBPath(db string) string { return "/dbs/" + url.PathEscape(db) }
func DBRID(db string) string  { return "dbs/" + db }
func CollPath(db, coll string) string {
	return DBPath(db) + "/colls/" + url.PathEscape(coll)
}
func CollRID(db, coll string) string { return DBRID(db) + "/colls/" + coll }
func DocsPath(db, coll string) string {
	return CollPath(db, coll) + "/docs"
}
func DocPath(db, coll, id string) string {
	return DocsPath(db, coll) + "/" + url.PathEscape(id)
}
func DocRID(db, coll, id string) string { return CollRID(db, coll) + "/docs/" + id }

// ---------------------------------------------------------------------------
// Feeds & pagination
// ---------------------------------------------------------------------------

// Feed fetches a collection feed (or query result set). envelope names the
// response's root array property (Databases / DocumentCollections / Documents
// / Offers). One page of `limit` items is fetched unless returnAll, which
// follows the opaque x-ms-continuation response header (echoed back verbatim
// as a request header) until it comes back empty or MaxAllPages is hit. body
// is re-sent on every page (queries POST the same body each time).
//
// The returned charge is the SUM of every page's x-ms-request-charge, so the
// RU number reported for a return-all matches what the account was billed.
func Feed(flow *core.Flow, a Auth, method, path, resourceType, resourceID, envelope string, headers map[string]string, body []byte, limit int, returnAll bool) ([]interface{}, string, error) {
	items := []interface{}{}
	var charge float64
	continuation := ""

	for page := 0; page < MaxAllPages; page++ {
		hdrs := map[string]string{"x-ms-max-item-count": strconv.Itoa(limit)}
		for k, v := range headers {
			hdrs[k] = v
		}
		if continuation != "" {
			hdrs["x-ms-continuation"] = continuation
		}

		resp, err := DoRequest(flow, a, method, path, resourceType, resourceID, hdrs, body)
		if err != nil {
			return nil, "", err
		}
		if err := CheckResponse(resp); err != nil {
			return nil, "", err
		}
		obj, err := DecodeObject(resp)
		if err != nil {
			return nil, "", err
		}
		if arr, ok := obj[envelope].([]interface{}); ok {
			items = append(items, arr...)
		}
		if c, err := strconv.ParseFloat(RequestCharge(resp), 64); err == nil {
			charge += c
		}

		continuation = resp.Headers.Get("x-ms-continuation")
		if !returnAll || continuation == "" {
			break
		}
	}
	return items, strconv.FormatFloat(charge, 'f', 2, 64), nil
}

// ---------------------------------------------------------------------------
// Partition keys
// ---------------------------------------------------------------------------

// pkCache remembers each container's partition-key path for the lifetime of
// one execution (the executor is a one-shot process), so a flow touching the
// same container twenty times pays the discovery GET once. Keyed by
// baseURL|db|coll — broader than the db|coll minimum so two accounts in one
// flow can never cross wires.
var pkCache = struct {
	mu sync.Mutex
	m  map[string]string
}{m: map[string]string{}}

// ContainerPartitionKeyPath returns the container's partition-key path (e.g.
// "/id" or "/category"), discovering it with a GET on the container the first
// time. Returns "" for a legacy non-partitioned container, which needs no
// partition-key header at all.
func ContainerPartitionKeyPath(flow *core.Flow, a Auth, db, coll string) (string, error) {
	key := a.BaseURL + "|" + db + "|" + coll
	pkCache.mu.Lock()
	if path, ok := pkCache.m[key]; ok {
		pkCache.mu.Unlock()
		return path, nil
	}
	pkCache.mu.Unlock()

	resp, err := DoRequest(flow, a, http.MethodGet, CollPath(db, coll), "colls", CollRID(db, coll), nil, nil)
	if err != nil {
		return "", err
	}
	if err := CheckResponse(resp); err != nil {
		return "", err
	}
	var def struct {
		PartitionKey struct {
			Paths []string `json:"paths"`
		} `json:"partitionKey"`
	}
	if err := json.Unmarshal(resp.Body, &def); err != nil {
		return "", fmt.Errorf("failed to parse container definition: %w", err)
	}
	path := ""
	if len(def.PartitionKey.Paths) > 0 {
		path = def.PartitionKey.Paths[0]
	}

	pkCache.mu.Lock()
	pkCache.m[key] = path
	pkCache.mu.Unlock()
	return path, nil
}

// PartitionKeyHeader renders a partition-key value as the JSON array literal
// the x-ms-documentdb-partitionkey header requires (["value"], [42], …). One
// value only — hierarchical (multi-path) partition keys are not supported.
func PartitionKeyHeader(v interface{}) string {
	b, err := json.Marshal([]interface{}{v})
	if err != nil {
		return `[""]`
	}
	return string(b)
}

// ResolvePointPartitionKey resolves the partition-key value for a point
// operation (get/patch/delete), where no item body is available: the explicit
// partition_key input wins; otherwise a container partitioned on /id derives
// it from the item id; a legacy non-partitioned container needs none (has =
// false). Anything else must be supplied by the operator.
//
// The explicit input short-circuits discovery, so flows that fill it in never
// pay the extra GET. Note the input is a string — an item partitioned on a
// NUMERIC value must be addressed via the item body forms instead (the same
// limitation n8n has).
func ResolvePointPartitionKey(flow *core.Flow, a Auth, inputs []*core.Connection, db, coll, itemID string) (interface{}, bool, error) {
	if v := OptionalString("partition_key", inputs); v != "" {
		return v, true, nil
	}
	path, err := ContainerPartitionKeyPath(flow, a, db, coll)
	if err != nil {
		return nil, false, err
	}
	switch {
	case path == "":
		return nil, false, nil
	case path == "/id" && itemID != "":
		return itemID, true, nil
	}
	return nil, false, fmt.Errorf("the container is partitioned on %q — set the Partition Key input to the item's value for it", path)
}

// ResolveBodyPartitionKey resolves the partition-key value for create/replace,
// where the item body is the natural source: the explicit partition_key input
// wins; otherwise the property at the container's partition-key path is read
// from the body; otherwise a /id container falls back to itemID. When the
// value came from outside the body and the body lacks the property (top-level
// paths only), it is injected so the document and the header cannot disagree.
func ResolveBodyPartitionKey(flow *core.Flow, a Auth, inputs []*core.Connection, db, coll, itemID string, body map[string]interface{}) (interface{}, bool, error) {
	path, err := ContainerPartitionKeyPath(flow, a, db, coll)
	if err != nil {
		return nil, false, err
	}
	if path == "" {
		return nil, false, nil
	}

	if v := OptionalString("partition_key", inputs); v != "" {
		ensurePartitionKeyInBody(body, path, v)
		return v, true, nil
	}
	if v, ok := lookupPath(body, path); ok {
		return v, true, nil
	}
	if path == "/id" && itemID != "" {
		ensurePartitionKeyInBody(body, path, itemID)
		return itemID, true, nil
	}
	return nil, false, fmt.Errorf("the container is partitioned on %q — include that property in the item, or set the Partition Key input", path)
}

// lookupPath walks a Cosmos partition-key path ("/a/b") through nested maps.
func lookupPath(body map[string]interface{}, path string) (interface{}, bool) {
	cur := interface{}(body)
	for _, seg := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		m, ok := cur.(map[string]interface{})
		if !ok {
			return nil, false
		}
		cur, ok = m[seg]
		if !ok {
			return nil, false
		}
	}
	return cur, cur != nil
}

// ensurePartitionKeyInBody injects the partition-key property when the body
// lacks it, for top-level (single-segment) paths only — nested paths are left
// for the server to validate.
func ensurePartitionKeyInBody(body map[string]interface{}, path string, v interface{}) {
	seg := strings.TrimPrefix(path, "/")
	if body == nil || seg == "" || strings.Contains(seg, "/") {
		return
	}
	if _, ok := body[seg]; !ok {
		body[seg] = v
	}
}

// ---------------------------------------------------------------------------
// Simplify
// ---------------------------------------------------------------------------

// Simplify strips Cosmos system properties — every key starting with "_"
// (_rid, _self, _etag, _attachments, _ts, …) — from a returned object.
func Simplify(obj map[string]interface{}) map[string]interface{} {
	out := make(map[string]interface{}, len(obj))
	for k, v := range obj {
		if strings.HasPrefix(k, "_") {
			continue
		}
		out[k] = v
	}
	return out
}

// SimplifyItems applies Simplify to each object in a feed when on is true.
func SimplifyItems(items []interface{}, on bool) []interface{} {
	if !on {
		return items
	}
	out := make([]interface{}, 0, len(items))
	for _, it := range items {
		if m, ok := it.(map[string]interface{}); ok {
			out = append(out, Simplify(m))
			continue
		}
		out = append(out, it)
	}
	return out
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

// BoolDefaultTrue extracts a boolean input that defaults to TRUE when the
// checkbox was never touched (simplify). An explicit false is respected.
func BoolDefaultTrue(name string, inputs []*core.Connection) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return true
	}
	return *conn.Boolean()
}

// OptionalJSON parses an object/array-typed input into an arbitrary value.
// Returns (nil, nil) when absent/blank, (nil, err) on malformed JSON.
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

// RequiredObject parses a required object-typed input into a map.
func RequiredObject(name string, inputs []*core.Connection) (map[string]interface{}, error) {
	v, err := OptionalJSON(name, inputs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf("%s is required", name)
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a JSON object, e.g. {"key":"value"}`, name)
	}
	return obj, nil
}

// RequiredArray parses a required object-typed input into a JSON array.
func RequiredArray(name string, inputs []*core.Connection) ([]interface{}, error) {
	v, err := OptionalJSON(name, inputs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf("%s is required", name)
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a JSON array, e.g. [{"op":"set","path":"/status","value":"done"}]`, name)
	}
	return arr, nil
}

// SetIfPresent adds an optional string field to a body only when the input was
// provided, so unset fields are omitted.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// SetIntIfPresent parses an optional string/integer input as an integer and
// adds it to the body when present. A non-numeric value is surfaced.
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

// ClampLimit bounds a requested page size to Cosmos's 1-1000 x-ms-max-item-count
// range, falling back to DefaultPageLimit when unset.
func ClampLimit(limit int, set bool) int {
	if !set || limit <= 0 {
		return DefaultPageLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit
	}
	return limit
}

// ThroughputHeaders builds the create-time provisioned-throughput headers
// shared by database_create and container_create: manual RU/s via
// x-ms-offer-throughput, or autoscale via x-ms-cosmos-offer-autopilot-setting.
// The autoscale header value is the JSON object {"maxThroughput":N} the
// service documents (n8n sends a bare number, which the service rejects).
// Mutually exclusive; both unset means the account/database default.
func ThroughputHeaders(inputs []*core.Connection) (map[string]string, error) {
	manual, hasManual := OptionalInt("throughput", inputs)
	autoscale, hasAutoscale := OptionalInt("autoscale_max", inputs)
	if hasManual && hasAutoscale {
		return nil, fmt.Errorf("throughput and autoscale_max are mutually exclusive — set one or the other")
	}
	headers := map[string]string{}
	if hasManual {
		if manual < 400 {
			return nil, fmt.Errorf("throughput must be at least 400 RU/s")
		}
		headers["x-ms-offer-throughput"] = strconv.Itoa(manual)
	}
	if hasAutoscale {
		if autoscale < 1000 {
			return nil, fmt.Errorf("autoscale_max must be at least 1000 RU/s")
		}
		setting, _ := json.Marshal(map[string]int{"maxThroughput": autoscale})
		headers["x-ms-cosmos-offer-autopilot-setting"] = string(setting)
	}
	return headers, nil
}

// QueryParameters shapes the parameters Object input (a {"@name": value} map)
// into Cosmos's [{"name","value"}] list, prefixing a missing "@" so a
// parameter typed without it still binds. Keys are sorted for determinism.
func QueryParameters(inputs []*core.Connection) ([]map[string]interface{}, error) {
	v, err := OptionalJSON("parameters", inputs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`parameters must be a JSON object mapping names to values, e.g. {"@status":"open"}`)
	}
	names := make([]string, 0, len(obj))
	for name := range obj {
		names = append(names, name)
	}
	sort.Strings(names)
	params := make([]map[string]interface{}, 0, len(names))
	for _, name := range names {
		wire := name
		if !strings.HasPrefix(wire, "@") {
			wire = "@" + wire
		}
		params = append(params, map[string]interface{}{"name": wire, "value": obj[name]})
	}
	return params, nil
}

// FindOffer resolves the standing throughput offer for a container: GET the
// container for its _rid, then query /offers for the offer whose
// offerResourceId matches. Returns the raw offer document (nil when the
// container has no dedicated offer — shared-database throughput or a
// serverless account) and the summed RU charge of the lookups. The offers
// feed/query signs with an EMPTY resourceId; only single-offer GET/PUT use the
// offer's lowercased _rid.
func FindOffer(flow *core.Flow, a Auth, db, coll string) (map[string]interface{}, string, error) {
	resp, err := DoRequest(flow, a, http.MethodGet, CollPath(db, coll), "colls", CollRID(db, coll), nil, nil)
	if err != nil {
		return nil, "", err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, "", err
	}
	collDef, err := DecodeObject(resp)
	if err != nil {
		return nil, "", err
	}
	collRid, _ := collDef["_rid"].(string)
	if collRid == "" {
		return nil, "", fmt.Errorf("container definition carries no _rid — cannot resolve its throughput offer")
	}

	query, _ := json.Marshal(map[string]interface{}{
		"query":      "SELECT * FROM root r WHERE r.offerResourceId = @rid",
		"parameters": []map[string]interface{}{{"name": "@rid", "value": collRid}},
	})
	headers := map[string]string{
		"Content-Type":            "application/query+json",
		"x-ms-documentdb-isquery": "True",
	}
	offers, feedCharge, err := Feed(flow, a, http.MethodPost, "/offers", "offers", "", "Offers", headers, query, DefaultPageLimit, true)
	if err != nil {
		return nil, "", err
	}
	charge := SumCharges(RequestCharge(resp), feedCharge)
	if len(offers) == 0 {
		return nil, charge, nil
	}
	offer, ok := offers[0].(map[string]interface{})
	if !ok {
		return nil, charge, fmt.Errorf("unexpected offer shape in /offers response")
	}
	return offer, charge, nil
}

// SumCharges adds RU charge strings (each may be ""), formatted to the two
// decimals the service itself uses.
func SumCharges(charges ...string) string {
	var total float64
	for _, c := range charges {
		if v, err := strconv.ParseFloat(c, 64); err == nil {
			total += v
		}
	}
	return strconv.FormatFloat(total, 'f', 2, 64)
}

// OfferPathAndRID derives the URL path and signing resourceId for a single
// offer. The path uses the offer's id; the signing resourceId is the offer's
// _rid LOWERCASED — the one place in the API where the signing id is not the
// raw resource link (an undocumented quirk the SDKs all encode).
func OfferPathAndRID(offer map[string]interface{}) (string, string, error) {
	rid, _ := offer["_rid"].(string)
	id, _ := offer["id"].(string)
	if id == "" {
		id = rid
	}
	if id == "" || rid == "" {
		return "", "", fmt.Errorf("offer carries no id/_rid — cannot address it")
	}
	return "/offers/" + url.PathEscape(id), strings.ToLower(rid), nil
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
// output. Cosmos ids are strings already; charge is the RU cost.
func ResourceResult(obj map[string]interface{}, charge, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	id, _ := obj["id"].(string)
	return map[string]interface{}{
		"id":             id,
		"result":         obj,
		"request_charge": charge,
		"tool_result":    summary,
		"success":        true,
		"error":          "",
	}
}

// ListResult shapes a feed response into the standard list output.
func ListResult(items []interface{}, charge, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":        items,
		"count":          len(items),
		"request_charge": charge,
		"tool_result":    summary,
		"success":        true,
		"error":          "",
	}
}
