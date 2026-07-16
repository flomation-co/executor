// Package entra holds the shared Microsoft Graph plumbing used by every
// azure/entra/* action: service-principal auth via the shared Azure
// client-credentials mint, request execution, the Graph error envelope, and
// @odata.nextLink pagination.
//
// Four things shape this file:
//
//   - Auth is app-only (OAuth2 client credentials). n8n's Entra node only
//     supports delegated authorization-code, which cannot run unattended —
//     app-only is the natural fit for directory automation. Tokens come from
//     azure.ClientCredentialsToken (actions/azure/common.go) with the scope
//     derived from the Graph endpoint, so a sovereign-cloud override
//     (graph.microsoft.us, dod-graph.microsoft.us, …) gets a token whose
//     audience matches the host it is sent to.
//   - Every list/search call sends ConsistencyLevel: eventual plus $count=true.
//     Graph rejects advanced $filter/$search (endsWith, filter-on-null, any
//     $search) with Request_UnsupportedQuery without the pair — n8n omits both
//     on its user-facing list ops, so those documented filters fail there.
//   - Return All follows @odata.nextLink VERBATIM. The link already encodes
//     the original $params plus an opaque $skiptoken; re-appending our own
//     query would corrupt the walk.
//   - Errors arrive as {error:{code,message}}; CheckResponse surfaces
//     "code: message" and friendly-maps the ones an operator will actually
//     hit: Request_ResourceNotFound, the already-a-member conflict, and the
//     license-assignment-needs-usageLocation failure.
//
// DELIBERATELY DESCOPED: the SharePoint-backed personal user properties
// (aboutMe, birthday, hireDate, interests, mySite, pastProjects,
// preferredName, responsibilities, schools, skills). Writing them is
// delegated-only — an app-only PATCH fails, which is why n8n requests
// Sites.FullControl.All and issues a second PATCH just for them. They are
// excluded from DefaultUserSelect for the same reason.
package entra

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	azure "flomation.app/automate/executor/actions/azure"
)

const (
	// DefaultGraphEndpoint is the global-cloud Microsoft Graph host. The
	// graph_endpoint auth input overrides it for sovereign clouds.
	DefaultGraphEndpoint = "https://graph.microsoft.com"

	// graphAPIPath pins Graph v1.0. Beta-only properties are unreachable by
	// design — the beta surface changes without notice.
	graphAPIPath = "/v1.0"

	// maxResponseBody caps a response read. Directory objects are small; a
	// 999-item user page is still well under 8 MB.
	maxResponseBody = 8 << 20 // 8 MB

	requestTimeout = 60 * time.Second

	// DefaultPageLimit / MaxPageLimit bound $top. Graph caps $top at 999 on
	// directory collections.
	DefaultPageLimit = 50
	MaxPageLimit     = 999

	// MaxAllPages bounds a Return All nextLink walk so a huge tenant can never
	// spin unbounded requests (100 pages × $top 999 ≈ 100k objects). On hitting
	// the cap the action says so in tool_result.
	MaxAllPages = 100

	// ODataBindChunk is Graph's per-request cap on directory-object references:
	// 20 for members@odata.bind on a group PATCH and 20 group ids per
	// checkMemberGroups call. Callers chunk with ChunkStrings.
	ODataBindChunk = 20
)

// httpClient is shared across every Entra action so connections to Graph (and
// to login.microsoftonline.com via the token mint) are pooled, not re-dialled
// per call.
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// reqContext returns the flow's cancellation context, or a background context
// when flow is nil (unit tests drive Execute with a nil flow).
func reqContext(flow *core.Flow) context.Context {
	if flow == nil {
		return context.Background()
	}
	return flow.GoContext()
}

// AuthInputs is the canonical credential block. Action packages re-declare
// their own literal Inputs arrays (the manifest generator AST-parses those
// literals, so it cannot see through a shared variable), but every action puts
// these four first, in this order. entra_inputs_drift_test.go enforces it.
//
// The names are azure_-prefixed so no resource field can collide:
// core.FindConnection returns the FIRST name match, and the credential block
// is declared first, so a resource input reusing a credential name would be
// silently shadowed.
var AuthInputs = []core.Connection{
	{
		Name:        "azure_tenant_id",
		Type:        core.ConnectionTypeString,
		Label:       "Tenant ID",
		Placeholder: "Directory (tenant) ID — a GUID or your-tenant.onmicrosoft.com",
		Required:    true,
	},
	{
		Name:        "azure_client_id",
		Type:        core.ConnectionTypeString,
		Label:       "Client ID",
		Placeholder: "Application (client) ID of the app registration",
		Required:    true,
	},
	{
		Name:        "azure_client_secret",
		Type:        core.ConnectionTypeSecret,
		Label:       "Client Secret",
		Placeholder: "App registration ▸ Certificates & secrets — the secret Value, not its ID",
		Required:    true,
	},
	{
		Name:        "graph_endpoint",
		Type:        core.ConnectionTypeString,
		Label:       "Graph Endpoint",
		Placeholder: "https://graph.microsoft.com — override for sovereign clouds (e.g. https://graph.microsoft.us)",
	},
}

// Auth is the resolved service principal plus the normalised Graph endpoint
// (scheme + host, no trailing slash, no /v1.0 suffix).
type Auth struct {
	TenantID     string
	ClientID     string
	ClientSecret string
	Endpoint     string
}

// BaseURL is the versioned API root every path is appended to.
func (a Auth) BaseURL() string { return a.Endpoint + graphAPIPath }

// Scope derives the token scope from the endpoint so a sovereign-cloud
// override mints a token audienced for that cloud, not the global one. For
// the default endpoint this is exactly https://graph.microsoft.com/.default.
func (a Auth) Scope() string { return a.Endpoint + "/.default" }

// GetAuth resolves the service principal and Graph endpoint from the action's
// auth inputs. A missing credential is a hard failure (zero Auth + real
// error) — there is nothing to attempt without it.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	tenant, err := RequiredString("azure_tenant_id", inputs)
	if err != nil {
		return Auth{}, err
	}
	client, err := RequiredString("azure_client_id", inputs)
	if err != nil {
		return Auth{}, err
	}
	secret, err := RequiredString("azure_client_secret", inputs)
	if err != nil {
		return Auth{}, err
	}
	endpoint, err := normaliseEndpoint(OptionalString("graph_endpoint", inputs))
	if err != nil {
		return Auth{}, err
	}
	return Auth{TenantID: tenant, ClientID: client, ClientSecret: secret, Endpoint: endpoint}, nil
}

// normaliseEndpoint reduces the graph_endpoint override to scheme+host with no
// trailing slash and no version suffix, defaulting to the global cloud.
func normaliseEndpoint(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return DefaultGraphEndpoint, nil
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("graph_endpoint is not a valid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("graph_endpoint must be an http(s) URL, e.g. https://graph.microsoft.us")
	}
	if u.Host == "" {
		return "", fmt.Errorf("graph_endpoint must include a host, e.g. https://graph.microsoft.us")
	}
	path := strings.TrimRight(u.Path, "/")
	path = strings.TrimSuffix(path, graphAPIPath)
	return u.Scheme + "://" + u.Host + path, nil
}

// acquireToken indirects the token exchange so tests can stub it; the real
// path is the shared per-execution cache in actions/azure/common.go.
var acquireToken = func(ctx context.Context, a Auth) (string, error) {
	return azure.ClientCredentialsToken(ctx, httpClient, a.TenantID, a.ClientID, a.ClientSecret, a.Scope())
}

// SetTokenForTest bypasses the real Entra token exchange, handing every
// request the given bearer token, and returns a restore function. Test-only.
func SetTokenForTest(token string) func() {
	prev := acquireToken
	acquireToken = func(context.Context, Auth) (string, error) { return token, nil }
	return func() { acquireToken = prev }
}

// APIResponse wraps an HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ExecuteAPI performs one Graph call. path is relative to {endpoint}/v1.0 and
// carries its own query string when it needs one; body (when non-nil) is
// marshalled to JSON. Advanced-query headers are NOT sent — list ops go
// through ListAll, which sends them.
func ExecuteAPI(flow *core.Flow, a Auth, method, path string, body interface{}) (*APIResponse, error) {
	return executeURL(flow, a, method, a.BaseURL()+path, body, false)
}

// executeURL is the single wire path: mint/fetch the bearer token, issue the
// request under the flow's context, read a capped body. advancedQuery adds the
// ConsistencyLevel: eventual header that pairs with $count=true on list ops.
func executeURL(flow *core.Flow, a Auth, method, fullURL string, body interface{}, advancedQuery bool) (*APIResponse, error) {
	ctx := reqContext(flow)
	token, err := acquireToken(ctx, a)
	if err != nil {
		// azure.ClientCredentialsToken already redacts the client secret, but
		// scrub again — this string ends up in ErrorResult output.
		return nil, fmt.Errorf("%s", redact(a, "", err.Error()))
	}

	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", redact(a, token, err.Error()))
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if bodyReader != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if advancedQuery {
		req.Header.Set("ConsistencyLevel", "eventual")
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Microsoft Graph request failed: %s", redact(a, token, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// redact scrubs the client secret and the minted bearer token from an error
// message before it reaches ErrorResult output or logs.
func redact(a Auth, token, msg string) string {
	msg = azure.RedactSecret(msg, a.ClientSecret)
	return azure.RedactSecret(msg, token)
}

// CheckResponse verifies a 2xx status, decoding Graph's {error:{code,message}}
// envelope into "code: message" and friendly-mapping the failures an operator
// will actually hit.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	var env struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(resp.Body, &env); err == nil && (env.Error.Code != "" || env.Error.Message != "") {
		return fmt.Errorf("Microsoft Graph error (%d): %s", resp.StatusCode, friendlyGraphError(env.Error.Code, env.Error.Message))
	}
	body := string(resp.Body)
	if len(body) > 500 {
		body = body[:500]
	}
	return fmt.Errorf("Microsoft Graph error (%d): %s", resp.StatusCode, body)
}

// friendlyGraphError appends a plain-language hint to the Graph errors whose
// raw text does not tell a non-technical operator what to do next.
func friendlyGraphError(code, message string) string {
	msg := message
	if code != "" {
		msg = code + ": " + message
	}
	lower := strings.ToLower(message)
	switch {
	case code == "Request_ResourceNotFound":
		return msg + " — no object with that ID exists in this tenant; check the ID/UPN (a recently deleted user or group can be restored with Restore Deleted Object)"
	case strings.Contains(lower, "added object references already exist"):
		return msg + " — the user is already a member of the group"
	case strings.Contains(lower, "invalid usage location"):
		return msg + ` — set usageLocation on the user first (Update User with update fields {"usageLocation":"GB"})`
	}
	return msg
}

// listEnvelope is Graph's collection shape: items under "value", the next
// page (when any) as an absolute opaque URL.
type listEnvelope struct {
	Value    []interface{} `json:"value"`
	NextLink string        `json:"@odata.nextLink"`
}

// ListAll fetches a Graph collection. Every request carries
// ConsistencyLevel: eventual and $count=true so advanced $filter/$search work.
// When returnAll, @odata.nextLink is followed verbatim (it already encodes the
// $params) up to MaxAllPages. nextLink is non-empty when more pages remained —
// either a single-page fetch that was not exhaustive, or a Return All walk
// that hit the page cap.
func ListAll(flow *core.Flow, a Auth, path string, q url.Values, returnAll bool) (items []interface{}, nextLink string, err error) {
	if q == nil {
		q = url.Values{}
	}
	q.Set("$count", "true")
	return listPages(flow, a, a.BaseURL()+path+"?"+q.Encode(), returnAll, true)
}

// ListSimple fetches a collection WITHOUT the advanced-query pair, for the
// handful of endpoints that reject unsupported query params outright
// (/subscribedSkus supports only $select — no $count, no paging).
func ListSimple(flow *core.Flow, a Auth, path string, q url.Values) ([]interface{}, error) {
	fullURL := a.BaseURL() + path
	if len(q) > 0 {
		fullURL += "?" + q.Encode()
	}
	items, _, err := listPages(flow, a, fullURL, true, false)
	return items, err
}

func listPages(flow *core.Flow, a Auth, fullURL string, returnAll, advancedQuery bool) ([]interface{}, string, error) {
	items := []interface{}{}
	pages := 0
	for {
		resp, err := executeURL(flow, a, http.MethodGet, fullURL, nil, advancedQuery)
		if err != nil {
			return nil, "", err
		}
		if err := CheckResponse(resp); err != nil {
			return nil, "", err
		}
		var env listEnvelope
		if err := json.Unmarshal(resp.Body, &env); err != nil {
			return nil, "", fmt.Errorf("failed to parse Microsoft Graph list response: %w", err)
		}
		items = append(items, env.Value...)
		pages++
		if env.NextLink == "" {
			return items, "", nil
		}
		if !returnAll || pages >= MaxAllPages {
			return items, env.NextLink, nil
		}
		fullURL = env.NextLink // verbatim — do not re-append $params
	}
}

// ApplyPaging sets $top from the limit input and reports whether return_all is
// on. A Return All walk pins $top to the 999 maximum so it makes as few round
// trips as Graph allows; the limit input then only bounds a single-page fetch.
func ApplyPaging(q url.Values, inputs []*core.Connection) bool {
	returnAll := OptionalBool("return_all", inputs)
	if returnAll {
		q.Set("$top", strconv.Itoa(MaxPageLimit))
	} else {
		limit, set := OptionalInt("limit", inputs)
		q.Set("$top", strconv.Itoa(ClampLimit(limit, set)))
	}
	return returnAll
}

// ListSummary phrases the standard list tool_result. capped means a Return All
// walk stopped at the MaxAllPages safety cap with pages still remaining.
func ListSummary(noun string, count int, returnAll, capped bool) string {
	if returnAll && capped {
		return fmt.Sprintf("Fetched %d %s(s); stopped at the %d-page safety cap — narrow the filter to get the rest", count, noun, MaxAllPages)
	}
	if returnAll {
		return fmt.Sprintf("Fetched all %d %s(s)", count, noun)
	}
	return fmt.Sprintf("Found %d %s(s)", count, noun)
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

// BoolOrDefault extracts a boolean input, returning def when the checkbox was
// never touched — used for default-on toggles (account_enabled,
// send_invitation) where nil must mean true, not false.
func BoolOrDefault(name string, inputs []*core.Connection, def bool) bool {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Boolean() == nil {
		return def
	}
	return *conn.Boolean()
}

// OptionalJSON parses an object-typed input into an arbitrary value. Returns
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

// SetIfPresent adds an optional string field to a request body only when the
// input was provided, so unset fields are omitted from the PATCH/POST.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// SetBoolIfSet adds an optional boolean field when its input connection is
// present (checkbox touched), so the tri-state nil is preserved as "omit" —
// essential on PATCH, where sending false is not the same as not sending.
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
// onto the request body — the escape hatch for any Graph property not exposed
// as a first-class input (givenName, surname, jobTitle, usageLocation, …).
//
// It is called LAST in every action's body assembly, so a key here OVERRIDES
// the same key set by a first-class input. This "power-user last word"
// precedence is deliberate and matches the WordPress / WooCommerce nodes.
func MergeAdditionalFields(body map[string]interface{}, inputs []*core.Connection) error {
	return MergeObjectInput(body, inputs, "additional_fields")
}

// MergeObjectInput overlays the named JSON-object input onto body, erroring on
// malformed JSON or a non-object shape. Used for additional_fields and the
// update actions' update_fields.
func MergeObjectInput(body map[string]interface{}, inputs []*core.Connection, name string) error {
	v, err := OptionalJSON(name, inputs)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Errorf(`%s must be a JSON object, e.g. {"key":"value"}`, name)
	}
	for k, val := range obj {
		body[k] = val
	}
	return nil
}

// SplitCommaList splits a comma-separated input into trimmed non-empty parts.
func SplitCommaList(raw string) []string {
	out := []string{}
	for _, part := range strings.Split(raw, ",") {
		if p := strings.TrimSpace(part); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ChunkStrings splits a list into slices of at most size — used for Graph's
// 20-reference cap on members@odata.bind and checkMemberGroups.
func ChunkStrings(list []string, size int) [][]string {
	if size <= 0 {
		size = ODataBindChunk
	}
	chunks := [][]string{}
	for len(list) > size {
		chunks = append(chunks, list[:size])
		list = list[size:]
	}
	if len(list) > 0 {
		chunks = append(chunks, list)
	}
	return chunks
}

// ClampLimit bounds a requested $top to Graph's 1-999 range, falling back to
// DefaultPageLimit when unset.
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
// Client-side validation (mirrors n8n's preSend checks, so a bad value fails
// with a named field instead of an opaque Graph 400)
// ---------------------------------------------------------------------------

// upnCharset is the character set Graph accepts in a userPrincipalName.
var upnCharset = regexp.MustCompile(`^[A-Za-z0-9'._!#^~@-]+$`)

// ValidateUPN checks the userPrincipalName charset and shape.
func ValidateUPN(upn string) error {
	if !strings.Contains(upn, "@") {
		return fmt.Errorf("user_principal_name must be a full UPN like jane@your-tenant.onmicrosoft.com")
	}
	if !upnCharset.MatchString(upn) {
		return fmt.Errorf("user_principal_name contains characters Graph does not accept (allowed: letters, digits, ' . - _ ! # ^ ~ and one @)")
	}
	return nil
}

// ValidateDisplayName enforces Graph's 256-character displayName cap.
func ValidateDisplayName(name string) error {
	if len(name) > 256 {
		return fmt.Errorf("display_name must be 256 characters or fewer (got %d)", len(name))
	}
	return nil
}

// ValidateMailNickname enforces the mail alias rules: local part only (no @),
// 64 chars max, ASCII only.
func ValidateMailNickname(nick string) error {
	if len(nick) > 64 {
		return fmt.Errorf("mail_nickname must be 64 characters or fewer (got %d)", len(nick))
	}
	if strings.Contains(nick, "@") {
		return fmt.Errorf("mail_nickname is the local part only — no @ (e.g. jane.doe, not jane.doe@contoso.com)")
	}
	for i := 0; i < len(nick); i++ {
		if nick[i] > 127 || nick[i] == ' ' {
			return fmt.Errorf("mail_nickname must be ASCII with no spaces")
		}
	}
	return nil
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

// ResourceResult shapes a single-object Graph response into the standard
// action output. id is lifted from the object (Graph ids are strings).
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	id := ""
	if v, ok := obj["id"].(string); ok {
		id = v
	} else if obj["id"] != nil {
		id = fmt.Sprintf("%v", obj["id"])
	}
	return map[string]interface{}{
		"id":          id,
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// EchoResult shapes a 204-No-Content mutation into the standard output: Graph
// returned nothing, so result echoes what was done.
func EchoResult(id string, echo map[string]interface{}, summary string) map[string]interface{} {
	if echo == nil {
		echo = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          id,
		"result":      echo,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a collection response into the standard list output. A
// non-nil empty slice serialises as [] not null (get-many feeds Loop nodes).
func ListResult(items []interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []interface{}{}
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// Decode unmarshals a successful single-object body into a generic map.
func Decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Microsoft Graph response: %w", err)
	}
	return out, nil
}

// DefaultUserSelect is the rich $select sent by Get User when the operator
// leaves Select Fields blank. Graph's GET /users/{id} returns only a small
// default property subset unless $select is explicit, so parity of output
// richness requires naming everything (n8n's "Raw" output does the same with
// ~60 properties). The SharePoint-backed personal properties are deliberately
// absent — see the package comment.
const DefaultUserSelect = "id,accountEnabled,ageGroup,assignedLicenses,assignedPlans,businessPhones,city," +
	"companyName,consentProvidedForMinor,country,createdDateTime,creationType,department,displayName," +
	"employeeHireDate,employeeId,employeeOrgData,employeeType,externalUserState,externalUserStateChangeDateTime," +
	"faxNumber,givenName,identities,imAddresses,jobTitle,lastPasswordChangeDateTime,legalAgeGroupClassification," +
	"licenseAssignmentStates,mail,mailNickname,mobilePhone,officeLocation,onPremisesDistinguishedName," +
	"onPremisesDomainName,onPremisesExtensionAttributes,onPremisesImmutableId,onPremisesLastSyncDateTime," +
	"onPremisesProvisioningErrors,onPremisesSamAccountName,onPremisesSecurityIdentifier,onPremisesSyncEnabled," +
	"onPremisesUserPrincipalName,otherMails,passwordPolicies,postalCode,preferredDataLocation,preferredLanguage," +
	"provisionedPlans,proxyAddresses,securityIdentifier,showInAddressList,state,streetAddress,surname," +
	"usageLocation,userPrincipalName,userType"
