// Package azuredevops holds the shared HTTP client, auth, request builder and
// result plumbing used by every devops/azuredevops/* action.
//
// This node is hand-rolled REST rather than azure-devops-go-api, deliberately.
// The SDK's entire auth value-add is `"Basic " + base64(":"+pat)` — there is no
// signing and no protocol for a library to own — and it resolves every URL
// through a location service, costing up to three HTTP round trips on a cold
// process. Our executor is one-shot (spawned per run, then dies), so every
// process is cold and would pay that amplification on every node. Its last
// functional release is 7.1 from April 2023, so depending on it would also pin
// us there with no route to 7.2. One `http.Request` per call is both faster and
// less code.
//
// Four Azure DevOps constraints are handled here so the actions don't have to:
//
//   - `?api-version=` is mandatory on EVERY request. Omitting it does not error
//     cleanly — you get a legacy response shape or a redirect. Execute bakes it
//     in, so no call site can forget. A few endpoints are still preview-only in
//     7.1 (work item comments); Request.APIVersion overrides per call.
//   - A bad or expired PAT does NOT return 401. Azure DevOps answers 203
//     Non-Authoritative Information with an HTML sign-in page, which a naive
//     `>= 400` check reads as success and then fails downstream with an
//     incomprehensible unmarshal error. CheckResponse treats 203 (and an HTML
//     body on any status) as an auth failure by name.
//   - "The org URL" is a lie: resource areas live on different subdomains.
//     Releases are served from vsrm.dev.azure.com, everything we use otherwise
//     from dev.azure.com. Requests name their host explicitly (HostCore /
//     HostRelease); a release call sent to the core host 404s.
//   - Continuation tokens come back in the `x-ms-continuationtoken` RESPONSE
//     HEADER, not the body, and are echoed as a `continuationToken` query
//     param. Some API versions return the header even on the last page, so the
//     pager loops on "zero items returned", never on "header absent" — the
//     other way round is an infinite pager.
//
// Auth is a PAT over HTTP Basic with an EMPTY username: base64(":" + pat). The
// leading colon is mandatory; base64 of the bare token fails. Azure DevOps
// OAuth is dead (no new app registrations since April 2025), so the only
// Connect-button path would be Entra ID OAuth — a follow-up, not this wave.
//
// PAT scopes for this action set: vso.work_write (work items, comments, WIQL),
// vso.build_execute (pipelines, builds, artifacts), vso.code_write (repos, PRs,
// commits), vso.project (projects, teams), vso.release_execute (releases).
package azuredevops

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	azure "flomation.app/automate/executor/actions/azure"
)

const (
	// DefaultAPIVersion is the version every request carries unless an action
	// overrides it. Bumping this is a one-constant change — which is most of the
	// reason this node is not built on the SDK.
	DefaultAPIVersion = "7.1"

	// CommentsAPIVersion pins the ONLY endpoints in this node still shipping as
	// preview in 7.1: work item comments (GET/POST .../workItems/{id}/comments).
	//
	// This is a live time-bomb, not a stylistic choice. Microsoft deactivates a
	// preview version ~12 weeks after the API is released, and a request naming
	// a dead -preview version is REJECTED rather than silently upgraded. If the
	// two comment actions start failing with a version error, comments have
	// gone GA and this constant should be deleted, not bumped.
	CommentsAPIVersion = "7.1-preview.4"

	// maxResponseBody caps a response read. Build logs are the large case, hence
	// 8 MB rather than 1 MB; a clipped log is reported as truncated rather than
	// passed off as complete.
	maxResponseBody = 8 << 20 // 8 MB

	requestTimeout = 60 * time.Second

	// DefaultPageLimit / MaxPageLimit bound $top on the paged list endpoints.
	DefaultPageLimit = 50
	MaxPageLimit     = 1000

	// MaxAllPages bounds a Return All continuation walk so a large organisation
	// can never spin unbounded requests. On hitting it the action says so in
	// tool_result rather than quietly returning a partial list.
	MaxAllPages = 50

	// WorkItemBatchLimit is the server-side cap on ids per workitemsbatch call.
	// Exceeding it is a 400, so callers chunk.
	WorkItemBatchLimit = 200

	// MaxWiqlResults bounds how many WIQL references we hydrate. A WIQL query
	// resolving more than 20,000 items fails server-side outright (VS402337),
	// and every 200 references costs another round trip, so an unbounded query
	// would be both slow and useless in a flow.
	MaxWiqlResults = 2000
)

// httpClient is shared across every Azure DevOps action so connections to the
// organisation are pooled and reused rather than re-dialled per call.
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

// SleepOrCancel waits for d, returning early with an error if the flow is
// cancelled. A bare time.Sleep would leave a cancelled flow's executor sitting
// in a polling loop nobody is waiting on any more.
func SleepOrCancel(flow *core.Flow, d time.Duration) error {
	ctx := reqContext(flow)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(d):
		return nil
	}
}

// ---------------------------------------------------------------------------
// Credentials
// ---------------------------------------------------------------------------

// AuthInputs is the canonical credential block. Action packages re-declare
// their own literal Inputs arrays (the manifest generator AST-parses those
// literals and cannot see through a shared variable), but every action puts
// these three first, in this order. azuredevops_inputs_drift_test.go is the
// enforcement.
//
// The names are reserved: core.FindConnection returns the FIRST name match and
// the credential block is declared first, so a resource field reusing one of
// these names would silently read the credential instead.
var AuthInputs = []core.Connection{
	{
		Name:        "organisation_url",
		Type:        core.ConnectionTypeString,
		Label:       "Organisation URL",
		Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)",
		Required:    true,
	},
	{
		Name:        "personal_access_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Personal Access Token",
		Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation",
		Required:    true,
	},
	{
		Name:        "api_version",
		Type:        core.ConnectionTypeString,
		Label:       "API Version",
		Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version",
	},
}

// Auth is a resolved Azure DevOps credential for one action invocation.
type Auth struct {
	// CoreBase is the organisation root on dev.azure.com, no trailing slash.
	CoreBase string
	// ReleaseBase is the same organisation on the vsrm host, no trailing slash.
	ReleaseBase string
	PAT         string
	APIVersion  string
}

// Host selects which of Azure DevOps' subdomains a request is routed to.
type Host int

const (
	// HostCore is dev.azure.com — projects, git, build, pipelines, work items.
	HostCore Host = iota
	// HostRelease is vsrm.dev.azure.com — classic Release Management only.
	HostRelease
)

// GetAuth resolves and validates the credential block.
func GetAuth(inputs []*core.Connection) (Auth, error) {
	raw := OptionalString("organisation_url", inputs)
	coreBase, releaseBase, err := normaliseOrgURL(raw)
	if err != nil {
		return Auth{}, err
	}
	pat := OptionalString("personal_access_token", inputs)
	if pat == "" {
		return Auth{}, fmt.Errorf("Personal Access Token is required")
	}
	version := OptionalString("api_version", inputs)
	if version == "" {
		version = DefaultAPIVersion
	}
	return Auth{CoreBase: coreBase, ReleaseBase: releaseBase, PAT: pat, APIVersion: version}, nil
}

// normaliseOrgURL reduces a pasted organisation URL to the core base and its
// vsrm twin.
//
// Two URL shapes are live in the wild and both must work: the modern
// https://dev.azure.com/{org} and the legacy https://{org}.visualstudio.com,
// which Microsoft still serves. Their Release hosts differ in shape as well as
// name (vsrm.dev.azure.com/{org} vs {org}.vsrm.visualstudio.com), which is why
// this is derived once here rather than string-patched per call.
//
// The host is lower-cased because DNS is case-insensitive; the PATH is NOT.
// (The SDK lower-cases the whole URL, which is wrong — project names in paths
// are case-sensitive. Do not copy that.) Query, fragment and any user:pass@
// smuggled into the pasted value are dropped so a crafted URL cannot append
// itself to every request.
func normaliseOrgURL(raw string) (coreBase, releaseBase string, err error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", fmt.Errorf("Organisation URL is required, e.g. https://dev.azure.com/your-org")
	}
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, parseErr := url.Parse(s)
	if parseErr != nil || u.Host == "" {
		return "", "", fmt.Errorf("Organisation URL must be a full http(s) URL, e.g. https://dev.azure.com/your-org")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", "", fmt.Errorf("Organisation URL must start with http:// or https://")
	}
	u.User = nil
	u.RawQuery = ""
	u.Fragment = ""
	host := strings.ToLower(u.Host)
	path := strings.TrimRight(u.Path, "/")

	coreBase = u.Scheme + "://" + host + path

	switch {
	case host == "dev.azure.com" || strings.HasSuffix(host, ".dev.azure.com"):
		if path == "" {
			return "", "", fmt.Errorf("Organisation URL must include the organisation, e.g. https://dev.azure.com/your-org")
		}
		releaseBase = u.Scheme + "://vsrm.dev.azure.com" + path
	case strings.HasSuffix(host, ".visualstudio.com"):
		org := strings.TrimSuffix(host, ".visualstudio.com")
		if org == "" || strings.Contains(org, ".") {
			return "", "", fmt.Errorf("Organisation URL must be https://your-org.visualstudio.com")
		}
		releaseBase = u.Scheme + "://" + org + ".vsrm.visualstudio.com" + path
	default:
		// Azure DevOps Server (on-prem): collection topology varies and there is
		// no vsrm split — releases are served from the same host. Hardcoding the
		// cloud's three hosts is what every real-world client does; this is the
		// branch to revisit if on-prem ever becomes a supported target.
		releaseBase = coreBase
	}
	return coreBase, releaseBase, nil
}

// basicAuth builds the Authorization value. The username is EMPTY and the
// leading colon is mandatory — base64 of the bare PAT is rejected.
func basicAuth(pat string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(":"+pat))
}

// redact scrubs the PAT and its base64 form from a message before it reaches
// ErrorResult output or a log. Both forms matter: a transport error can quote
// the request headers, which carry the encoded credential, not the raw one.
func redact(a Auth, msg string) string {
	msg = azure.RedactSecret(msg, a.PAT)
	if a.PAT != "" {
		msg = azure.RedactSecret(msg, base64.StdEncoding.EncodeToString([]byte(":"+a.PAT)))
	}
	return msg
}

// ---------------------------------------------------------------------------
// HTTP
// ---------------------------------------------------------------------------

// Request describes one Azure DevOps call. Path is rooted at the organisation
// (e.g. "/_apis/projects" or "/"+project+"/_apis/pipelines") and must NOT
// carry a query string — Query owns that, so Execute can add api-version
// without string-patching.
type Request struct {
	Method string
	Path   string
	Query  url.Values
	Host   Host

	// Body is marshalled to JSON. Ignored when RawBody is set.
	Body interface{}
	// RawBody + ContentType carry a pre-encoded body, used for the JSON-Patch
	// documents work item create/update require.
	RawBody     []byte
	ContentType string

	// APIVersion overrides the connection's version for the endpoints that are
	// still preview-only (see CommentsAPIVersion).
	APIVersion string
}

// Response is the raw outcome of a call. Continuation carries the
// x-ms-continuationtoken response header; Truncated is set when the body hit
// maxResponseBody, so a log fetch reports incompleteness rather than passing a
// clipped log off as the whole thing.
type Response struct {
	StatusCode   int
	Body         []byte
	Header       http.Header
	ContentType  string
	Continuation string
	Truncated    bool
}

// Execute performs one Azure DevOps call, adding auth and the mandatory
// api-version, and routing to the host the request names.
func Do(flow *core.Flow, a Auth, r Request) (*Response, error) {
	base := a.CoreBase
	if r.Host == HostRelease {
		base = a.ReleaseBase
	}

	q := url.Values{}
	for k, vs := range r.Query {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	version := r.APIVersion
	if version == "" {
		version = a.APIVersion
	}
	q.Set("api-version", version)

	var reader io.Reader
	contentType := r.ContentType
	switch {
	case r.RawBody != nil:
		reader = bytes.NewReader(r.RawBody)
	case r.Body != nil:
		b, err := json.Marshal(r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reader = bytes.NewReader(b)
		if contentType == "" {
			contentType = "application/json"
		}
	}

	req, err := http.NewRequestWithContext(reqContext(flow), r.Method, base+r.Path+"?"+q.Encode(), reader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %s", redact(a, err.Error()))
	}
	req.Header.Set("Authorization", basicAuth(a.PAT))
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Azure DevOps request failed: %s", redact(a, err.Error()))
	}
	defer func() { _ = resp.Body.Close() }()

	// Read one byte past the cap so an exactly-at-cap body is distinguishable
	// from a clipped one.
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %s", redact(a, err.Error()))
	}
	truncated := false
	if len(body) > maxResponseBody {
		body = body[:maxResponseBody]
		truncated = true
	}
	return &Response{
		StatusCode:   resp.StatusCode,
		Body:         body,
		Header:       resp.Header,
		ContentType:  resp.Header.Get("Content-Type"),
		Continuation: resp.Header.Get("x-ms-continuationtoken"),
		Truncated:    truncated,
	}, nil
}

// CheckResponse verifies the status is 2xx and that the body is not a sign-in
// page, decoding Azure DevOps' error envelope into a usable message.
//
// The 203 branch is the important one. Azure DevOps answers a bad or expired
// PAT with 203 Non-Authoritative Information and an HTML sign-in page, NOT 401
// — so a plain status check treats the most common credential failure there is
// as a success and hands HTML to a JSON decoder.
func CheckResponse(resp *Response, acceptable ...int) error {
	if resp.StatusCode == http.StatusNonAuthoritativeInfo || looksLikeSignInPage(resp) {
		return fmt.Errorf("Azure DevOps returned a sign-in page instead of data (HTTP %d) — the Personal Access Token is missing, expired, or lacks the required scope", resp.StatusCode)
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	for _, code := range acceptable {
		if resp.StatusCode == code {
			return nil
		}
	}

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("Azure DevOps rejected the credentials (401) — check the Personal Access Token has not expired")
	case http.StatusForbidden:
		return fmt.Errorf("Azure DevOps refused the request (403): %s — the Personal Access Token authenticated but lacks the scope or permission for this operation", apiMessage(resp))
	case http.StatusNotFound:
		return fmt.Errorf("Azure DevOps returned 404 Not Found: %s — check the Organisation URL, the project, and that the resource exists", apiMessage(resp))
	}
	if msg := apiMessage(resp); msg != "" {
		return fmt.Errorf("Azure DevOps API error (%d): %s", resp.StatusCode, msg)
	}
	return fmt.Errorf("Azure DevOps API error (%d)", resp.StatusCode)
}

// looksLikeSignInPage detects the HTML the auth redirect serves. Content-Type
// alone is not enough to key on — build logs come back as text/plain and are
// perfectly valid — so this only fires on an actual HTML document.
func looksLikeSignInPage(resp *Response) bool {
	if strings.Contains(strings.ToLower(resp.ContentType), "text/html") {
		return true
	}
	head := strings.ToLower(strings.TrimSpace(string(resp.Body)))
	if len(head) > 200 {
		head = head[:200]
	}
	return strings.HasPrefix(head, "<!doctype html") || strings.HasPrefix(head, "<html")
}

// apiMessage pulls the human-readable text out of Azure DevOps' error envelope
// ({message, typeKey, errorCode, ...}), falling back to a clipped body.
func apiMessage(resp *Response) string {
	var env struct {
		Message string `json:"message"`
		TypeKey string `json:"typeKey"`
	}
	if json.Unmarshal(resp.Body, &env) == nil && env.Message != "" {
		return strings.Join(strings.Fields(env.Message), " ")
	}
	s := strings.Join(strings.Fields(string(resp.Body)), " ")
	if len(s) > 400 {
		s = s[:400] + "…"
	}
	return s
}

// Decode unmarshals a successful single-object body into a generic map.
func Decode(resp *Response) (map[string]interface{}, error) {
	if len(resp.Body) == 0 {
		return map[string]interface{}{}, nil
	}
	out := map[string]interface{}{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse the Azure DevOps response: %w", err)
	}
	return out, nil
}

// listEnvelope is Azure DevOps' collection shape. Note the continuation token
// is NOT in here — it arrives as a response header.
type listEnvelope struct {
	Count int           `json:"count"`
	Value []interface{} `json:"value"`
}

// DecodeList unmarshals a {count, value:[…]} collection body.
func DecodeList(resp *Response) ([]interface{}, error) {
	if len(resp.Body) == 0 {
		return []interface{}{}, nil
	}
	var env listEnvelope
	if err := json.Unmarshal(resp.Body, &env); err != nil {
		return nil, fmt.Errorf("failed to parse the Azure DevOps list response: %w", err)
	}
	if env.Value == nil {
		return []interface{}{}, nil
	}
	return env.Value, nil
}

// ListAll fetches a collection, following continuation tokens when returnAll.
// capped reports that the walk stopped at MaxAllPages with pages remaining, so
// the caller can say so rather than present a partial list as complete.
//
// The loop termination is deliberate: it stops when a page returns zero items,
// NOT when the continuation header is absent. Some API versions return the
// header on the last page too, and trusting it would loop forever.
func ListAll(flow *core.Flow, a Auth, r Request, returnAll bool) (items []interface{}, capped bool, err error) {
	items = []interface{}{}
	if r.Query == nil {
		r.Query = url.Values{}
	}
	for page := 0; ; page++ {
		resp, err := Do(flow, a, r)
		if err != nil {
			return nil, false, err
		}
		if err := CheckResponse(resp); err != nil {
			return nil, false, err
		}
		batch, err := DecodeList(resp)
		if err != nil {
			return nil, false, err
		}
		items = append(items, batch...)
		if !returnAll || len(batch) == 0 || resp.Continuation == "" {
			return items, false, nil
		}
		if page+1 >= MaxAllPages {
			return items, true, nil
		}
		// The token is opaque: do not trim, unquote or re-encode it. url.Values
		// percent-encodes it on the way out, which is required — it commonly
		// contains +, / and =.
		q := url.Values{}
		for k, vs := range r.Query {
			q[k] = vs
		}
		q.Set("continuationToken", resp.Continuation)
		r.Query = q
	}
}

// ---------------------------------------------------------------------------
// JSON-Patch for work items
// ---------------------------------------------------------------------------

// PatchOp is one operation in an Azure DevOps JSON-Patch document.
//
// The omitempty on Value is load-bearing, and subtler than it looks. On an
// interface{} field encoding/json omits ONLY a nil interface — never Go's zero
// values — so this drops "value" on exactly the remove op (whose Value is nil,
// and which RFC 6902 says must not carry one) while a field set to "", 0 or
// false still marshals with its value intact.
//
// That distinction is not academic: Azure DevOps rejects a valueless operation
// with `400 Value cannot be null.`, naming neither the field nor the value. So
// if this ever did omit zero values, emptying a description or setting a
// priority to 0 would fail with an error nobody could act on. Do NOT "simplify"
// Value to a concrete type — a string would take omitempty's empty-string
// branch and introduce precisely that bug.
type PatchOp struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

// JSONPatchContentType is the content type work item create/update require.
// Sending application/json instead yields a 400 that does not name the cause.
const JSONPatchContentType = "application/json-patch+json"

// fieldAliases maps the plain-English names an operator types onto Azure
// DevOps' dotted reference names.
//
// This exists because the API addresses fields by reference name only —
// System.Title, Microsoft.VSTS.Common.Priority — never by the friendly label
// the web UI shows. Asking a front-of-house operator to know that (let alone to
// hand-write a JSON-Patch document) is the wrong trade, so the field map input
// accepts either: a key containing a "." is passed through as a reference name,
// anything else is looked up here.
//
// An unknown alias is an ERROR, not a guess. Inventing "System.<Key>" would
// produce a 400 from the server naming a field the operator never typed.
var fieldAliases = map[string]string{
	"title":               "System.Title",
	"description":         "System.Description",
	"state":               "System.State",
	"reason":              "System.Reason",
	"assigned to":         "System.AssignedTo",
	"assigned_to":         "System.AssignedTo",
	"assignee":            "System.AssignedTo",
	"area path":           "System.AreaPath",
	"area_path":           "System.AreaPath",
	"area":                "System.AreaPath",
	"iteration path":      "System.IterationPath",
	"iteration_path":      "System.IterationPath",
	"iteration":           "System.IterationPath",
	"sprint":              "System.IterationPath",
	"tags":                "System.Tags",
	"history":             "System.History",
	"comment":             "System.History",
	"priority":            "Microsoft.VSTS.Common.Priority",
	"severity":            "Microsoft.VSTS.Common.Severity",
	"acceptance criteria": "Microsoft.VSTS.Common.AcceptanceCriteria",
	"acceptance_criteria": "Microsoft.VSTS.Common.AcceptanceCriteria",
	"repro steps":         "Microsoft.VSTS.TCM.ReproSteps",
	"repro_steps":         "Microsoft.VSTS.TCM.ReproSteps",
	"steps to reproduce":  "Microsoft.VSTS.TCM.ReproSteps",
	"story points":        "Microsoft.VSTS.Scheduling.StoryPoints",
	"story_points":        "Microsoft.VSTS.Scheduling.StoryPoints",
	"effort":              "Microsoft.VSTS.Scheduling.Effort",
	"remaining work":      "Microsoft.VSTS.Scheduling.RemainingWork",
	"remaining_work":      "Microsoft.VSTS.Scheduling.RemainingWork",
	"original estimate":   "Microsoft.VSTS.Scheduling.OriginalEstimate",
	"original_estimate":   "Microsoft.VSTS.Scheduling.OriginalEstimate",
	"completed work":      "Microsoft.VSTS.Scheduling.CompletedWork",
	"completed_work":      "Microsoft.VSTS.Scheduling.CompletedWork",
	"start date":          "Microsoft.VSTS.Scheduling.StartDate",
	"start_date":          "Microsoft.VSTS.Scheduling.StartDate",
	"target date":         "Microsoft.VSTS.Scheduling.TargetDate",
	"target_date":         "Microsoft.VSTS.Scheduling.TargetDate",
	"due date":            "Microsoft.VSTS.Scheduling.DueDate",
	"due_date":            "Microsoft.VSTS.Scheduling.DueDate",
	"value area":          "Microsoft.VSTS.Common.ValueArea",
	"value_area":          "Microsoft.VSTS.Common.ValueArea",
	"activity":            "Microsoft.VSTS.Common.Activity",
	"risk":                "Microsoft.VSTS.Common.Risk",
	"business value":      "Microsoft.VSTS.Common.BusinessValue",
	"business_value":      "Microsoft.VSTS.Common.BusinessValue",
	"system info":         "Microsoft.VSTS.TCM.SystemInfo",
	"system_info":         "Microsoft.VSTS.TCM.SystemInfo",
}

// ResolveFieldName turns an operator-typed key into an Azure DevOps reference
// name. A key already containing a "." is assumed to be one and passes through
// verbatim (case is preserved — reference names are matched case-insensitively
// by the service, but echoing what was typed keeps errors legible).
func ResolveFieldName(key string) (string, error) {
	k := strings.TrimSpace(key)
	if k == "" {
		return "", fmt.Errorf("field names cannot be blank")
	}
	if strings.Contains(k, ".") {
		return k, nil
	}
	if ref, ok := fieldAliases[strings.ToLower(k)]; ok {
		return ref, nil
	}
	return "", fmt.Errorf("unknown field %q — use its Azure DevOps reference name (e.g. System.Title, Microsoft.VSTS.Common.Priority), "+
		"or one of the shorthands: title, description, state, reason, assigned to, area path, iteration path, tags, priority, severity, story points, effort", key)
}

// escapePointer escapes a JSON-Pointer segment per RFC 6901. Reference names
// contain neither ~ nor /, but a raw path escape hatch could, and a wrong
// pointer fails as an unrelated-looking server error.
func escapePointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	return strings.ReplaceAll(s, "/", "~1")
}

// FieldPatch translates a friendly field map into the JSON-Patch document work
// item create/update require. This is the whole point of the two actions: an
// operator supplies {"title": "Bug in checkout", "priority": 1} and never sees
// [{"op":"add","path":"/fields/System.Title","value":"Bug in checkout"}].
//
// Three rules, each earning its place:
//
//   - op is ALWAYS "add", never "replace". "add" is set-or-replace here;
//     "replace" fails outright on a field with no current value, which is the
//     classic trap — an update that works on one work item and 400s on the next
//     purely because that field happened to be empty.
//   - A null value becomes an explicit "remove", so a field map CAN clear a
//     field. Omitting the key leaves the field untouched.
//   - A key starting with "/" is used as a raw JSON-Pointer path (e.g.
//     "/relations/-" to append a link). This is the escape hatch for the parts
//     of the patch surface that are not /fields/*, and it is why the friendly
//     map does not have to grow a special case per relation type.
//
// Keys are sorted so the document — and therefore every test and every logged
// request — is deterministic; Go map iteration order is not.
func FieldPatch(fields map[string]interface{}) ([]PatchOp, error) {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	ops := make([]PatchOp, 0, len(fields))
	for _, key := range keys {
		value := fields[key]
		path := ""
		if strings.HasPrefix(strings.TrimSpace(key), "/") {
			path = strings.TrimSpace(key)
		} else {
			ref, err := ResolveFieldName(key)
			if err != nil {
				return nil, err
			}
			path = "/fields/" + escapePointer(ref)
		}
		if value == nil {
			ops = append(ops, PatchOp{Op: "remove", Path: path})
			continue
		}
		ops = append(ops, PatchOp{Op: "add", Path: path, Value: value})
	}
	if len(ops) == 0 {
		return nil, fmt.Errorf("no fields supplied — a work item patch needs at least one field")
	}
	return ops, nil
}

// EncodePatch marshals a patch document for the wire.
func EncodePatch(ops []PatchOp) ([]byte, error) {
	b, err := json.Marshal(ops)
	if err != nil {
		return nil, fmt.Errorf("failed to encode the work item patch: %w", err)
	}
	return b, nil
}

// ---------------------------------------------------------------------------
// Input helpers
// ---------------------------------------------------------------------------

// OptionalString extracts a string input, returning "" if absent. The explicit
// Value==nil guard matters for the Secret and Text types: unlike String, their
// String() renders a nil value as the literal "<nil>", so an unset field would
// otherwise read as non-empty.
func OptionalString(name string, inputs []*core.Connection) string {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil || conn.String() == nil {
		return ""
	}
	return strings.TrimSpace(*conn.String())
}

// RequiredString extracts a required string input, erroring if absent/blank.
func RequiredString(name, label string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", label)
	}
	return v, nil
}

// OptionalInt extracts an integer input. The bool is false when absent, so
// callers distinguish "unset" from "set to 0".
//
// The type switch guards a sharp edge in core Connection.Number(): its final
// fallback is an unchecked string assertion, which panics when a whole-value
// ${...} reference lands a slice/map/bool in an integer-typed input (e.g. an
// operator wires "Limit" to an upstream array). Only values Number() can handle
// reach it; anything else reads as unset rather than crashing the one-shot
// executor.
func OptionalInt(name string, inputs []*core.Connection) (int, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil || conn.Value == nil {
		return 0, false
	}
	switch v := conn.Value.(type) {
	case int, int64, float64:
	case string:
		if strings.TrimSpace(v) == "" {
			return 0, false
		}
	default:
		return 0, false
	}
	n := conn.Number()
	if n == nil {
		return 0, false
	}
	return int(*n), true
}

// RequiredInt extracts a required integer input.
func RequiredInt(name, label string, inputs []*core.Connection) (int, error) {
	v, ok := OptionalInt(name, inputs)
	if !ok {
		return 0, fmt.Errorf("%s is required", label)
	}
	return v, nil
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
// never touched — used for default-on toggles where nil must mean true.
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

// ObjectInput parses a JSON-object input. Returns nil when absent.
func ObjectInput(name, label string, inputs []*core.Connection) (map[string]interface{}, error) {
	v, err := OptionalJSON(name, inputs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a JSON object, e.g. {"key":"value"}`, label)
	}
	return obj, nil
}

// SetIfPresent adds an optional string field to a request body only when the
// input was provided, so unset fields are omitted from the POST/PATCH.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// SetBoolIfSet adds an optional boolean field when its checkbox was touched, so
// the tri-state nil is preserved as "omit" — essential on PATCH, where sending
// false is not the same as not sending.
func SetBoolIfSet(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	conn := core.FindConnection(inputName, inputs)
	if conn != nil && conn.Boolean() != nil {
		body[field] = *conn.Boolean()
	}
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

// ParseIDList parses a comma-separated list of integer ids, naming the first
// bad entry rather than silently dropping it.
func ParseIDList(raw, label string) ([]int, error) {
	parts := SplitCommaList(raw)
	if len(parts) == 0 {
		return nil, fmt.Errorf("%s is required — supply one or more ids, e.g. 42,43,44", label)
	}
	ids := make([]int, 0, len(parts))
	for _, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nil, fmt.Errorf("%s: %q is not a work item id — supply whole numbers, e.g. 42,43,44", label, p)
		}
		ids = append(ids, n)
	}
	return ids, nil
}

// ClampLimit bounds a requested $top, falling back to DefaultPageLimit.
func ClampLimit(limit int, set bool) int {
	if !set || limit <= 0 {
		return DefaultPageLimit
	}
	if limit > MaxPageLimit {
		return MaxPageLimit
	}
	return limit
}

// ApplyPaging sets $top from the limit input and reports whether return_all is
// on. A Return All walk pins $top to the maximum so it makes as few round trips
// as the service allows.
func ApplyPaging(q url.Values, inputs []*core.Connection) bool {
	returnAll := OptionalBool("return_all", inputs)
	if returnAll {
		q.Set("$top", strconv.Itoa(MaxPageLimit))
		return true
	}
	limit, set := OptionalInt("limit", inputs)
	q.Set("$top", strconv.Itoa(ClampLimit(limit, set)))
	return false
}

// FullRefName expands a bare branch name into the full ref Azure DevOps
// requires. The Git APIs take refs/heads/main and silently 400 on "main", which
// is one of the more common first-run failures — so accept either.
func FullRefName(branch string) string {
	b := strings.TrimSpace(branch)
	if b == "" {
		return ""
	}
	if strings.HasPrefix(b, "refs/") {
		return b
	}
	return "refs/heads/" + b
}

// ProjectPath returns the project segment of a URL, path-escaped. Project names
// are case-sensitive and routinely contain spaces.
func ProjectPath(project string) string {
	return "/" + url.PathEscape(project)
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ErrorResult is the standard soft-failure output (returned with a nil error so
// the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// ResourceResult shapes a single-object response into the standard action
// output, lifting id out of the object. Azure DevOps ids are integers on work
// items/builds/PRs and GUIDs on projects/repos, so both are stringified.
func ResourceResult(obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	return map[string]interface{}{
		"id":          IDOf(obj),
		"result":      obj,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// IDOf renders an object's id as a string. JSON numbers decode as float64, so a
// work item id 42 must not stringify as "42" via %v on a float — that yields
// "42" only by luck of formatting; strconv on the integral value is explicit.
func IDOf(obj map[string]interface{}) string {
	v, ok := obj["id"]
	if !ok || v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	case int:
		return strconv.Itoa(t)
	case int64:
		return strconv.FormatInt(t, 10)
	default:
		return fmt.Sprintf("%v", t)
	}
}

// ListResult shapes a collection into the standard list output. A non-nil empty
// slice serialises as [] not null (get-many feeds Loop nodes).
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

// SuccessResult is the standard output for an action whose only signal is that
// it worked (delete, cancel). extra keys are merged in.
func SuccessResult(id string, echo map[string]interface{}, summary string) map[string]interface{} {
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

// NormalisePipelineVariables translates a friendly {"name": "value"} map into
// the {"name": {"value": "…"}} envelope the pipelines API requires.
//
// The wire format exists so a variable can also carry isSecret, but an operator
// setting a build variable should not have to know that — a bare scalar is what
// they will type, and sending it unwrapped is a 400. A value that is already an
// object is passed through untouched, so the full envelope stays reachable for
// {"value":"x","isSecret":true}.
func NormalisePipelineVariables(in map[string]interface{}) map[string]interface{} {
	if in == nil {
		return nil
	}
	out := make(map[string]interface{}, len(in))
	for k, v := range in {
		if obj, ok := v.(map[string]interface{}); ok {
			out[k] = obj
			continue
		}
		out[k] = map[string]interface{}{"value": v}
	}
	return out
}

// ChunkInts splits a list of ids into slices of at most size — used for the
// 200-id cap on workitemsbatch.
func ChunkInts(list []int, size int) [][]int {
	if size <= 0 {
		size = WorkItemBatchLimit
	}
	chunks := [][]int{}
	for len(list) > size {
		chunks = append(chunks, list[:size])
		list = list[size:]
	}
	if len(list) > 0 {
		chunks = append(chunks, list)
	}
	return chunks
}

// FetchWorkItems hydrates work item ids via POST /wit/workitemsbatch, chunking
// at the server's 200-id cap. project may be blank for an organisation-scoped
// lookup. fields (reference names) and expand are mutually exclusive — the
// service rejects both together, so the caller must not set both.
//
// This is a shared helper because it is BOTH its own action and the engine
// behind workitem_query_wiql: WIQL returns only {id, url} references, so a
// query is worthless in a flow until its results are hydrated here.
func FetchWorkItems(flow *core.Flow, a Auth, project string, ids []int, fields []string, expand string) ([]interface{}, error) {
	items := []interface{}{}
	path := "/_apis/wit/workitemsbatch"
	if project != "" {
		path = ProjectPath(project) + path
	}
	for _, chunk := range ChunkInts(ids, WorkItemBatchLimit) {
		body := map[string]interface{}{"ids": chunk}
		if len(fields) > 0 {
			body["fields"] = fields
		} else if expand != "" && expand != "none" {
			body["$expand"] = expand
		}
		resp, err := Do(flow, a, Request{Method: http.MethodPost, Path: path, Body: body})
		if err != nil {
			return nil, err
		}
		if err := CheckResponse(resp); err != nil {
			return nil, err
		}
		batch, err := DecodeList(resp)
		if err != nil {
			return nil, err
		}
		items = append(items, batch...)
	}
	return items, nil
}
