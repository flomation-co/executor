// Package salesforce holds the shared HTTP client, auth helpers, SOQL query
// builder and generic record CRUD used by every crm/salesforce/* action.
//
// Salesforce's REST API is uniform across every sObject — standard objects
// (Lead, Contact, Account) and custom objects (Invoice__c) share identical
// create/read/update/delete shapes under /services/data/{version}/sobjects/{type}.
// That regularity lets the CRUD live here once (CreateRecord, GetRecord,
// UpdateRecord, DeleteRecord, UpsertRecord, Query) so each action package stays
// thin: read its inputs, call one helper, shape the result.
//
// Three Salesforce-specific traps shape this file and are worth knowing before
// changing anything in it:
//
//   - Errors are a JSON ARRAY, not an object. Salesforce returns
//     [{"message":"...","errorCode":"...","fields":["Email"]}] — a shape no
//     other Flomation integration uses. CheckResponse decodes that array and
//     translates the errorCode into something a non-technical operator can act
//     on, because the raw codes (FIELD_CUSTOM_VALIDATION_EXCEPTION,
//     INVALID_CROSS_REFERENCE_KEY) mean nothing to the person reading them.
//
//   - 204 No Content is the NORMAL success response for update and delete —
//     verified live on API v62. decode must tolerate an empty body, and those
//     helpers return the record ID they already hold rather than an empty map,
//     or nothing downstream can chain off an update, which is the single most
//     common flow shape. Upsert is the exception worth knowing: on v62 it
//     answers 201 {created:true} on insert and 200 {created:false} on match,
//     both WITH a body. Older API versions answered a matched upsert with a
//     bare 204, so both shapes are handled — but do not expect the 204.
//
//   - Every SOQL string is assembled, never parameterised: Salesforce has no
//     bind-variable syntax over REST. Identifiers are whitelist-validated and
//     never quoted; values are escaped and quoted. The guards below are ported
//     from n8n's node (escape order matters — backslash first, or every
//     subsequent escape doubles up) and are the only thing standing between a
//     filter value and SOQL injection.
//
// Auth is a Salesforce access token (ConnectionTypeSecret, so it accepts both a
// managed ${credentials.X} from the "Connect Salesforce" flow and a pasted
// ${secrets.X}) plus the org's instance URL, which cannot be derived: it is
// per-org, differs between production and sandbox, and changes on My Domain
// setup or org migration.
package salesforce

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	core "flomation.app/automate/executor"
)

const (
	// APIVersion pins the Salesforce REST API version. Salesforce ships three
	// releases a year and supports each version for roughly three years before
	// it returns 410 GONE (v21-v30 were retired in Summer '25).
	//
	// It is deliberately a single constant rather than a per-action input,
	// mirroring shopify.APIVersion — a version selector on every node would
	// just clutter the UI for non-technical users. It is pinned conservatively
	// rather than to the newest release because Salesforce staggers upgrades:
	// for several weeks after each release a large share of orgs are still on
	// the previous version and reject calls to a version they have not been
	// upgraded to yet. Every capability this node uses exists well below v62
	// (sObject Collections v42, Composite v38, parameterizedSearch v36).
	APIVersion = "62.0"

	// SOAPVersion pins the Partner SOAP API version used by the handful of
	// operations that have no REST equivalent (convertLead, merge, undelete).
	// Kept a separate const from APIVersion so the two can diverge — the SOAP
	// endpoints are far more stable and rarely need moving.
	SOAPVersion = "62.0"

	// maxResponseBody caps the response body to prevent memory exhaustion. A
	// return-all query over a large object can be big, so 8 MB (the airtable /
	// shopify value) rather than the 1 MB used by single-record integrations.
	maxResponseBody = 8 << 20 // 8 MB

	// requestTimeout is the HTTP client timeout for a single Salesforce call.
	requestTimeout = 60 * time.Second

	// MaxAllPages bounds a "return all" pagination loop so a query over a large
	// org can never spin unbounded requests. On hitting the cap the action
	// returns the outstanding nextRecordsUrl so the caller can resume.
	MaxAllPages = 100

	// DefaultPageLimit / MaxPageLimit bound the LIMIT clause on a single-page
	// query. Salesforce's own ceiling is 2000 records per page.
	DefaultPageLimit = 50
	MaxPageLimit     = 2000

	// MaxCollectionRecords is Salesforce's hard cap on the sObject Collections
	// endpoints (/composite/sobjects): 200 records per request. The bulk
	// helpers chunk automatically at this size, which is why an operator can
	// hand record_create_many a 1000-row array without knowing the limit
	// exists. Above roughly 2000 records Bulk API 2.0 is the correct tool and
	// this node deliberately does not pretend otherwise.
	MaxCollectionRecords = 200
)

// httpClient is shared across every Salesforce action so TCP connections to the
// org are pooled and reused rather than re-dialled per call (a flow run — or a
// return-all query — can fire many requests).
var httpClient = &http.Client{
	Timeout: requestTimeout,
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 20,
		IdleConnTimeout:     90 * time.Second,
	},
}

// AuthInputs is the shared credential shape. Action packages re-declare their
// own literal Inputs arrays (the manifest generator AST-parses those, so they
// cannot reference this var), but this documents the canonical pair every
// action puts first.
//
// access_token is ConnectionTypeSecret rather than ConnectionTypeCredential on
// purpose: a Secret slot accepts BOTH a managed ${credentials.X} and a pasted
// ${secrets.X}, while a Credential slot accepts only the former. One input
// therefore covers the managed "Connect Salesforce" flow and the
// bring-your-own-token path without the operator having to understand which
// they have.
var AuthInputs = []core.Connection{
	{
		Name:        "access_token",
		Type:        core.ConnectionTypeSecret,
		Label:       "Salesforce Connection",
		Placeholder: "Connect Salesforce, or paste an access token",
		Required:    true,
	},
	{
		Name:        "instance_url",
		Type:        core.ConnectionTypeString,
		Label:       "Salesforce Instance URL",
		Placeholder: "https://mycompany.my.salesforce.com",
		Required:    true,
	},
}

// APIResponse wraps the HTTP response for consistent handling.
type APIResponse struct {
	StatusCode int
	Body       []byte
	Headers    http.Header
}

// ---------------------------------------------------------------------------
// Instance URL handling
// ---------------------------------------------------------------------------

// salesforceHostSuffixes are the domains a Salesforce org can legitimately live
// on. The instance URL is operator-supplied and is used to build every request
// URL, so it is validated against this list before the access token is ever
// attached — otherwise a crafted instance_url would exfiltrate the token to an
// arbitrary host on the first action run. Same guard, same reason, as
// shopify.NormaliseShop's charset check.
var salesforceHostSuffixes = []string{
	".salesforce.com", // mycompany.my.salesforce.com, na1.salesforce.com
	".force.com",      // mycompany.lightning.force.com, *.develop.my.site.com aliases
	".salesforce.mil", // Government Cloud Plus
	".cloudforce.com", // legacy pods still in service
}

// testBaseURL, when set, replaces the operator's instance URL for every request
// AND relaxes host validation, so action packages in sibling directories can
// exercise Execute end-to-end against an httptest server. Test-only; the same
// seam idiom as shopify.SetHostForTest.
var testBaseURL string

// SetHostForTest redirects every request to the given base URL (an httptest
// server) and returns a function that restores real behaviour. Test-only.
func SetHostForTest(base string) func() {
	prev := testBaseURL
	testBaseURL = strings.TrimRight(base, "/")
	return func() { testBaseURL = prev }
}

// NormaliseInstanceURL reduces whatever the operator pasted to a bare
// scheme+host origin. It accepts a bare host ("mycompany.my.salesforce.com"),
// a full URL, a Lightning URL with a path
// ("https://mycompany.lightning.force.com/lightning/o/Lead/list"), or a URL
// with a trailing slash, and returns "https://mycompany.my.salesforce.com".
//
// It deliberately does NOT validate — ValidateInstanceURL does that — so
// callers can normalise and report on the cleaned value.
func NormaliseInstanceURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	// Accept a bare host by giving it a scheme to parse against. Anything with
	// an explicit http:// is upgraded: Salesforce is https-only and silently
	// downgrading would send the token in clear.
	if !strings.Contains(s, "://") {
		s = "https://" + s
	}
	u, err := url.Parse(s)
	if err != nil || u.Host == "" {
		return ""
	}
	host := strings.ToLower(u.Host)
	// Strip any userinfo Salesforce would never use but a crafted value might
	// carry, and any explicit port (orgs are always on 443).
	if i := strings.LastIndex(host, "@"); i >= 0 {
		host = host[i+1:]
	}
	if i := strings.LastIndex(host, ":"); i >= 0 {
		// Only strip a port, not an IPv6 bracket group.
		if !strings.Contains(host[i:], "]") {
			host = host[:i]
		}
	}
	return "https://" + host
}

// ValidateInstanceURL confirms a normalised instance URL points at a Salesforce
// -owned host. Returns a clear operator-facing error when it does not.
func ValidateInstanceURL(normalised string) error {
	if normalised == "" {
		return fmt.Errorf("instance_url is required — the Salesforce URL for your org, e.g. https://mycompany.my.salesforce.com")
	}
	u, err := url.Parse(normalised)
	if err != nil || u.Host == "" {
		return fmt.Errorf("instance_url is not a valid URL — expected something like https://mycompany.my.salesforce.com")
	}
	host := strings.ToLower(u.Hostname())
	for _, suffix := range salesforceHostSuffixes {
		// Require a real subdomain, not a bare suffix match, so "salesforce.com"
		// alone and "evilsalesforce.com" are both rejected.
		if strings.HasSuffix(host, suffix) && len(host) > len(suffix) {
			return nil
		}
	}
	return fmt.Errorf("instance_url must be a Salesforce address ending in .salesforce.com or .force.com — got %q. Copy it from your browser's address bar while signed in to Salesforce", host)
}

// BuildURL assembles a full REST API URL for a path below the version root.
// path must start with "/" and is relative to /services/data/v{APIVersion}
// (e.g. "/sobjects/Lead" or "/query?q=...").
func BuildURL(instanceURL, path string) string {
	base := instanceURL
	if testBaseURL != "" {
		base = testBaseURL
	}
	return base + "/services/data/v" + APIVersion + path
}

// BuildAbsoluteURL assembles a URL for a path Salesforce handed back that is
// already rooted at the instance (nextRecordsUrl comes back as
// "/services/data/v62.0/query/01g...-2000", version included).
func BuildAbsoluteURL(instanceURL, path string) string {
	base := instanceURL
	if testBaseURL != "" {
		base = testBaseURL
	}
	return base + path
}

// ---------------------------------------------------------------------------
// Request / response
// ---------------------------------------------------------------------------

// ExecuteAPI performs a REST call against the org.
//
// The helper is named ExecuteAPI, not Execute: the manifest generator treats
// ANY package-level func Execute as an action and would emit a phantom entry
// for this package that breaks the build (see internal/manifestlint).
//
// method: GET, POST, PATCH, DELETE
// path:   path below the version root, including any query string
// body:   optional payload — marshalled to JSON for POST/PATCH, ignored otherwise
func ExecuteAPI(instanceURL, token, method, path string, body interface{}) (*APIResponse, error) {
	return executeURL(BuildURL(instanceURL, path), token, method, body)
}

// ExecuteAbsolute performs a REST call against a Salesforce-supplied absolute
// path (one that already carries its own /services/data/vNN prefix).
func ExecuteAbsolute(instanceURL, token, method, path string, body interface{}) (*APIResponse, error) {
	return executeURL(BuildAbsoluteURL(instanceURL, path), token, method, body)
}

func executeURL(fullURL, token, method string, body interface{}) (*APIResponse, error) {
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
		// url.Error embeds the request URL, which is safe (no token in it),
		// but scrub defensively in case a redirect put credentials in a query.
		return nil, fmt.Errorf("Salesforce API request failed: %w", redactError(err, token))
	}
	defer func() { _ = resp.Body.Close() }()

	// Read ONE byte past the cap so an oversized body is DETECTED rather than
	// silently truncated mid-JSON. Without this the decoder gets a cut-off
	// object and the operator sees "unexpected end of JSON input" — a Go error
	// string, on a flow that worked yesterday and broke as the data grew.
	// file_download and attachment_download already do this and say why; the
	// shared request path never got the same treatment.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	if len(respBody) > maxResponseBody {
		return nil, fmt.Errorf("Salesforce sent back more data than this step can handle (over %d MB). Choose fewer fields, lower the Limit, or add a filter to narrow the results", maxResponseBody>>20)
	}

	return &APIResponse{StatusCode: resp.StatusCode, Body: respBody, Headers: resp.Header}, nil
}

// redactError strips the access token from an error string. Transport errors
// can quote the request URL, and a misconfigured proxy or redirect could put
// the token somewhere it gets echoed back.
func redactError(err error, token string) error {
	if err == nil || token == "" {
		return err
	}
	msg := err.Error()
	if !strings.Contains(msg, token) {
		return err
	}
	return fmt.Errorf("%s", strings.ReplaceAll(msg, token, "********"))
}

// sfError is one entry of Salesforce's error array.
type sfError struct {
	Message   string   `json:"message"`
	ErrorCode string   `json:"errorCode"`
	Fields    []string `json:"fields"`
	// The sObject Collections endpoints spell the same thing "statusCode"
	// inside each per-record result, not "errorCode". Decoding only errorCode
	// meant every bulk failure lost both its plain-English translation and the
	// code itself, leaving the operator with Salesforce's bare prose.
	StatusCode string `json:"statusCode"`
}

// code returns whichever spelling of the error code this response used.
func (e sfError) code() string {
	if e.ErrorCode != "" {
		return e.ErrorCode
	}
	return e.StatusCode
}

// oauthError is the shape returned by the token/identity endpoints and by a
// handful of REST failures that predate the array envelope.
type oauthError struct {
	Error       string `json:"error"`
	Description string `json:"error_description"`
}

// CheckResponse verifies a 2xx status, decoding Salesforce's error envelope.
//
// Salesforce returns an ARRAY of {message, errorCode, fields} — unlike every
// other integration in this repo, which return an object. The errorCode is the
// useful part and the raw text is not: "FIELD_CUSTOM_VALIDATION_EXCEPTION"
// tells an operator nothing, whereas "your Salesforce administrator's
// validation rule rejected this" tells them who to ask. explainErrorCode does
// that translation for the codes that actually show up in practice.
func CheckResponse(resp *APIResponse) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	if msg := formatSalesforceErrors(resp.Body, resp.StatusCode); msg != "" {
		return fmt.Errorf("Salesforce API error (%d): %s", resp.StatusCode, msg)
	}

	// OAuth-shaped error ({"error":"invalid_grant",...}).
	var oe oauthError
	if json.Unmarshal(resp.Body, &oe) == nil && oe.Error != "" {
		detail := oe.Description
		if detail == "" {
			detail = oe.Error
		}
		if oe.Error == "invalid_grant" {
			return fmt.Errorf("Salesforce rejected the connection (%d): %s — the token has expired or was revoked; reconnect Salesforce. A sandbox refresh also invalidates existing connections", resp.StatusCode, detail)
		}
		return fmt.Errorf("Salesforce API error (%d): %s", resp.StatusCode, detail)
	}

	body := strings.TrimSpace(string(resp.Body))
	if len(body) > 500 {
		body = body[:500]
	}
	if body == "" {
		body = http.StatusText(resp.StatusCode)
	}
	return fmt.Errorf("Salesforce API error (%d): %s", resp.StatusCode, body)
}

// formatSalesforceErrors renders the error array into one operator-readable
// line. Returns "" when the body is not Salesforce's array envelope.
//
// The errorCode is ALWAYS carried into the message, even when a plain-language
// explanation replaces it as the headline. Dropping it broke real behaviour:
// lead_add_to_campaign decides whether to update an existing membership by
// looking for DUPLICATE_VALUE in the error, and because the code was formatted
// away whenever there was no explanation for it, that entire branch was dead
// and "Update Their Status if Already a Member" silently did nothing.
func formatSalesforceErrors(body []byte, status int) string {
	var errs []sfError
	if err := json.Unmarshal(body, &errs); err != nil || len(errs) == 0 {
		return ""
	}
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		msg := strings.TrimSpace(e.Message)
		if explained := explainErrorCode(e.ErrorCode, e.Fields, status); explained != "" {
			if msg != "" {
				msg = explained + " (" + msg + ")"
			} else {
				msg = explained
			}
		}
		if msg == "" {
			msg = e.ErrorCode
		}
		if len(e.Fields) > 0 && !strings.Contains(msg, strings.Join(e.Fields, ", ")) {
			msg += " — field(s): " + strings.Join(e.Fields, ", ")
		}
		if e.ErrorCode != "" && !strings.Contains(msg, e.ErrorCode) {
			msg += " [" + e.ErrorCode + "]"
		}
		if msg != "" {
			parts = append(parts, msg)
		}
	}
	return strings.Join(parts, "; ")
}

// ErrorHasCode reports whether a Salesforce error carries a given errorCode.
// Action packages use it to branch on a specific failure (a duplicate, a
// missing record) instead of pattern-matching Salesforce's prose, which
// changes between releases and between locales.
func ErrorHasCode(err error, code string) bool {
	return err != nil && strings.Contains(err.Error(), code)
}

// explainErrorCode turns a Salesforce errorCode into plain language. Only the
// codes an operator actually hits are translated; anything else falls through
// to Salesforce's own message, which is usually adequate once the code is not
// the first thing they read.
//
// status matters because Salesforce reuses codes across genuinely different
// faults, and the HTTP status is what separates them. Confirmed live:
//
//	DELETE a missing record   -> 404 INVALID_CROSS_REFERENCE_KEY "invalid cross reference id"
//	a bad lookup in a body    -> 400 INVALID_CROSS_REFERENCE_KEY
//	describe a missing object -> 404 NOT_FOUND "The requested resource does not exist"
//	upsert on a non-ext-ID    -> 404 NOT_FOUND "Provided external ID field does not exist..."
//
// Guessing wrong here is worse than saying nothing: telling an operator their
// record was deleted when they actually picked the wrong Match On Field sends
// them looking for a problem that does not exist. Where the status cannot
// disambiguate, this returns "" and lets Salesforce's own message stand.
func explainErrorCode(code string, fields []string, status int) string {
	switch code {
	case "REQUIRED_FIELD_MISSING":
		if len(fields) > 0 {
			return "a required Salesforce field was left empty"
		}
		return "a required Salesforce field was left empty"
	case "FIELD_CUSTOM_VALIDATION_EXCEPTION":
		return "your Salesforce administrator's validation rule rejected this record"
	case "DUPLICATES_DETECTED":
		return "Salesforce's duplicate rule blocked this record — a matching record already exists"
	case "FIELD_INTEGRITY_EXCEPTION":
		// Overwhelmingly this is the State/Country picklist: orgs created in
		// recent years validate address State and Country against a fixed
		// list, free text is refused, and Salesforce's standard list has no
		// sub-states for some countries at all (the United Kingdom among them,
		// verified live) — so for those the field simply cannot be filled in.
		return "your Salesforce org checks this value against a fixed list — for an address, State/Province and Country must match your org's own list, and some countries have no states to choose from at all"
	case "DUPLICATE_VALUE":
		return "Salesforce already has a matching record and refused to create another"
	case "INVALID_CROSS_REFERENCE_KEY":
		// On a 404 the request ADDRESSED a record that is not there; there is
		// no linked record involved, and pointing at the Owner/Parent boxes
		// sends the operator to the wrong field entirely.
		if status == http.StatusNotFound {
			return "no record with that ID exists in your Salesforce org — check the ID you supplied"
		}
		return "a linked record ID does not exist or is the wrong object type"
	case "INSUFFICIENT_ACCESS_ON_CROSS_REFERENCE_ENTITY":
		return "a linked record does not exist, or the connected Salesforce user cannot see it"
	case "MALFORMED_ID":
		return "that is not a valid Salesforce record ID (IDs are 15 or 18 characters)"
	case "ENTITY_IS_DELETED":
		return "the record no longer exists — it may have been deleted"
	case "NOT_FOUND":
		// Salesforce reuses NOT_FOUND for a missing OBJECT, a missing endpoint,
		// and — the costly one — an upsert keyed on a field that is not an
		// External ID ("Provided external ID field does not exist or is not
		// accessible: Phone"). Its own message says exactly which; prepending
		// "the record no longer exists" actively contradicts it. Say nothing
		// and let the real message through.
		return ""
	case "INSUFFICIENT_ACCESS", "INSUFFICIENT_ACCESS_OR_READONLY":
		return "the connected Salesforce user does not have permission to do this"
	case "REQUEST_LIMIT_EXCEEDED":
		return "your org has used up its daily Salesforce API allowance"
	case "INVALID_SESSION_ID":
		return "the Salesforce connection has expired or was revoked — reconnect Salesforce"
	case "INVALID_TYPE":
		return "that object is not available in your Salesforce org — the feature may not be enabled, or the connected user cannot see it"
	case "INVALID_FIELD":
		return "a field name is not valid on that object, or the connected user cannot see it"
	case "MALFORMED_QUERY":
		return "the query could not be understood by Salesforce"
	case "STRING_TOO_LONG":
		return "a value is longer than the Salesforce field allows"
	case "UNABLE_TO_LOCK_ROW":
		return "Salesforce could not lock the record — another process was updating it; retry"
	case "STORAGE_LIMIT_EXCEEDED":
		return "your Salesforce org is out of data storage"
	case "CANNOT_INSERT_UPDATE_ACTIVATE_ENTITY":
		// This is also the code Salesforce returns when the connected user
		// lacks the Marketing User permission and tries to create a Campaign —
		// the very first failure anyone hits wiring up the campaign actions.
		// Claiming a trigger or workflow did it sends them to Setup ▸ Flows
		// hunting for a rule that does not exist.
		return "your Salesforce org rejected this change — usually a missing permission on the connected user (creating Campaigns needs Marketing User), or an Apex trigger or workflow"
	case "INVALID_OPERATION":
		return "Salesforce rejected this operation for the record's current state"
	}
	return ""
}

// decode unmarshals a successful response body into a generic map.
//
// An empty body is normal, not exceptional: Salesforce answers a successful
// update or delete with 204 No Content (verified live on v62). Returning an
// empty map rather than erroring is what lets the write helpers substitute the
// record ID they already know. A matched upsert also answered 204 on older API
// versions, so that shape still has to be tolerated even though v62 returns a
// body.
func decode(resp *APIResponse) (map[string]interface{}, error) {
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return map[string]interface{}{}, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Salesforce response: %w", err)
	}
	return out, nil
}

// decodeArray unmarshals a successful response body that is a JSON array (the
// describe-global sub-lists and the Collections endpoints return these).
func decodeArray(resp *APIResponse) ([]interface{}, error) {
	if len(bytes.TrimSpace(resp.Body)) == 0 {
		return []interface{}{}, nil
	}
	var out []interface{}
	if err := json.Unmarshal(resp.Body, &out); err != nil {
		return nil, fmt.Errorf("failed to parse Salesforce response: %w", err)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Auth
// ---------------------------------------------------------------------------

// GetAuth resolves the org instance URL and access token from the action's auth
// inputs. A missing or non-Salesforce instance URL, or a missing token, is a
// hard failure (nil result + real error) rather than a soft error output —
// these are configuration mistakes, not provider failures.
func GetAuth(inputs []*core.Connection) (instanceURL, token string, err error) {
	token = OptionalString("access_token", inputs)
	if token == "" {
		return "", "", fmt.Errorf("access_token is required — connect Salesforce, or select a secret holding an access token")
	}

	// Under the test seam every request is redirected anyway, so skip the
	// host validation an httptest URL could never pass.
	if testBaseURL != "" {
		return testBaseURL, token, nil
	}

	raw := OptionalString("instance_url", inputs)
	// Guard BEFORE normalising, because ValidateInstanceURL quotes the host it
	// rejected back into its message — and if the operator has put the access
	// token in this box, that host IS the token, which would then travel out
	// through the node's error output into the execution log.
	if err := guardInstanceURLValue(raw, token); err != nil {
		return "", "", err
	}

	instanceURL = NormaliseInstanceURL(raw)
	if err := ValidateInstanceURL(instanceURL); err != nil {
		return "", "", err
	}
	return instanceURL, token, nil
}

// guardInstanceURLValue catches the two ways the Instance URL box ends up
// holding something that must never be echoed.
//
// Both inputs sit next to each other and both accept a variable, so binding the
// Salesforce Connection into the Instance URL is an easy slip — and the message
// that reports it is written into the flow's error output, where it is kept and
// displayed. A secret must not make that trip, so these cases are refused by
// NAME and the value itself is never quoted back.
func guardInstanceURLValue(raw, token string) error {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil // ValidateInstanceURL states the requirement.
	}
	if token != "" && v == token {
		return fmt.Errorf("the Salesforce Instance URL box is set to your Salesforce connection — that box wants your org's web address instead, e.g. https://mycompany.my.salesforce.com. The connection belongs only in the Salesforce Connection box above")
	}
	if looksLikeSalesforceToken(v) {
		return fmt.Errorf("the Salesforce Instance URL box looks like it holds an access token rather than an address — it wants your org's web address, e.g. https://mycompany.my.salesforce.com")
	}
	if strings.Contains(v, "${") {
		// An unresolved reference means the value never arrived. Naming the box is
		// enough; the reference text adds nothing and may name a secret.
		return fmt.Errorf("the Salesforce Instance URL still contains a variable that did not resolve — check the step it points at ran, or type the org address directly")
	}
	return nil
}

// looksLikeSalesforceToken recognises Salesforce's session-token shape without
// matching any plausible URL. Tokens are of the form 00D...!AQEAQ..., and the
// "!" is the giveaway: it is never present in a host name.
func looksLikeSalesforceToken(v string) bool {
	return strings.Contains(v, "!") && len(v) > 40
}

// ---------------------------------------------------------------------------
// SOQL construction — the injection boundary
// ---------------------------------------------------------------------------

// EscapeSOQLString escapes a value for use inside a quoted SOQL literal.
//
// ORDER MATTERS: the backslash replacement must run first, or the backslashes
// introduced by the later replacements get escaped a second time and the
// literal is corrupted. Ported from n8n's escapeSoqlString, which learned this
// the same way.
//
// https://developer.salesforce.com/docs/atlas.en-us.soql_sosl.meta/soql_sosl/sforce_api_calls_soql_select_quotedstringescapes.htm
func EscapeSOQLString(value string) string {
	r := strings.NewReplacer(
		`\`, `\\`,
		`'`, `\'`,
		`"`, `\"`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
		"\f", `\f`,
		"\b", `\b`,
	)
	return r.Replace(value)
}

// validFieldPattern matches a Salesforce field identifier, including
// relationship traversal (Account.Name, Account__r.Owner.Email) and every
// custom suffix (__c, __r, __x, __e, __b, __mdt, __Share, __History).
var validFieldPattern = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_]*(__[a-zA-Z]+)?(\.[a-zA-Z][a-zA-Z0-9_]*(__[a-zA-Z]+)?)*$`)

// validObjectPattern matches a Salesforce sObject name, including a namespace
// prefix (Namespace__MyObject__c) and every custom suffix.
var validObjectPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*(?:__[A-Za-z][A-Za-z0-9_]*)*$`)

// ValidateSOQLFieldName whitelist-validates a field identifier. Identifiers
// cannot be quoted in SOQL, so validation — not escaping — is the only defence
// available for them.
func ValidateSOQLFieldName(field string) (string, error) {
	field = strings.TrimSpace(field)
	if !validFieldPattern.MatchString(field) {
		return "", fmt.Errorf("%q is not a valid Salesforce field name", field)
	}
	return field, nil
}

// ValidateSOQLObjectName whitelist-validates an sObject name.
func ValidateSOQLObjectName(object string) (string, error) {
	object = strings.TrimSpace(object)
	if !validObjectPattern.MatchString(object) {
		return "", fmt.Errorf("%q is not a valid Salesforce object name", object)
	}
	return object, nil
}

// soqlOperators is the closed set of comparison operators a filter may use.
// A whitelist map, not a pattern, so nothing that is not literally one of
// these can reach the query string.
var soqlOperators = map[string]string{
	"EQUAL":    "=",
	"=":        "=",
	"!=":       "!=",
	"<>":       "<>",
	"<":        "<",
	"<=":       "<=",
	">":        ">",
	">=":       ">=",
	"LIKE":     "LIKE",
	"NOT LIKE": "NOT LIKE",
	"IN":       "IN",
	"NOT IN":   "NOT IN",
	"INCLUDES": "INCLUDES",
	"EXCLUDES": "EXCLUDES",
}

// ValidateSOQLOperator maps an operator to its canonical SOQL form, rejecting
// anything outside the whitelist. Whitespace is normalised so "not   like"
// resolves the same as "NOT LIKE".
func ValidateSOQLOperator(op string) (string, error) {
	normalised := strings.ToUpper(strings.Join(strings.Fields(op), " "))
	if v, ok := soqlOperators[normalised]; ok {
		return v, nil
	}
	return "", fmt.Errorf("%q is not a valid comparison — use one of =, !=, <, <=, >, >=, LIKE, NOT LIKE, IN, NOT IN, INCLUDES, EXCLUDES", op)
}

// salesforceDateLiterals are the bare SOQL date keywords, which must NOT be
// quoted or Salesforce compares against the literal string instead of the date
// range. https://developer.salesforce.com/docs/atlas.en-us.soql_sosl.meta/soql_sosl/sforce_api_calls_soql_select_dateformats.htm
var salesforceDateLiterals = map[string]bool{
	"YESTERDAY": true, "TODAY": true, "TOMORROW": true,
	"LAST_WEEK": true, "THIS_WEEK": true, "NEXT_WEEK": true,
	"LAST_MONTH": true, "THIS_MONTH": true, "NEXT_MONTH": true,
	"LAST_90_DAYS": true, "NEXT_90_DAYS": true,
	"THIS_QUARTER": true, "LAST_QUARTER": true, "NEXT_QUARTER": true,
	"THIS_YEAR": true, "LAST_YEAR": true, "NEXT_YEAR": true,
	"THIS_FISCAL_QUARTER": true, "LAST_FISCAL_QUARTER": true, "NEXT_FISCAL_QUARTER": true,
	"THIS_FISCAL_YEAR": true, "LAST_FISCAL_YEAR": true, "NEXT_FISCAL_YEAR": true,
}

var (
	relativeNDateLiteral = regexp.MustCompile(`^(LAST|NEXT)_N_(DAYS|WEEKS|MONTHS|QUARTERS|YEARS|FISCAL_QUARTERS|FISCAL_YEARS):\d+$`)
	agoNDateLiteral      = regexp.MustCompile(`^N_(DAYS|WEEKS|MONTHS|QUARTERS|YEARS|FISCAL_QUARTERS|FISCAL_YEARS)_AGO:\d+$`)
	isoDateTimePattern   = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,3})?(Z|[+-]\d{2}:\d{2})?$`)
	isoDatePattern       = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
)

// SOQLValue renders an operator-supplied filter value as a SOQL literal.
//
// The typing rules are Salesforce's, and getting them wrong produces a query
// that runs but matches nothing:
//
//   - date literals (TODAY, LAST_N_DAYS:7, N_DAYS_AGO:3) — uppercased, UNQUOTED
//   - ISO dates and datetimes — UNQUOTED
//   - booleans and real numbers — bare
//   - a comma-separated list, for IN / NOT IN — rendered as (a,b,c)
//   - everything else — escaped and quoted
//
// Numeric-looking STRINGS are quoted, matching n8n's typeVersion 1.1 fix: the
// input carries no field-type information, and a string-typed Salesforce field
// (an external ID, a postcode) needs a quoted literal regardless of content. An
// operator wanting a numeric comparison passes a number.
func SOQLValue(raw string, listValue bool) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "''", nil
	}

	if listValue {
		return soqlListValue(v)
	}

	upper := strings.ToUpper(v)
	if salesforceDateLiterals[upper] || relativeNDateLiteral.MatchString(upper) || agoNDateLiteral.MatchString(upper) {
		return upper, nil
	}
	if isoDateTimeNoOffset.MatchString(v) {
		// Salesforce rejects an offsetless datetime outright; assume UTC rather
		// than emit a literal that is guaranteed to fail.
		return v + "Z", nil
	}
	if isoDateTimePattern.MatchString(v) || isoDatePattern.MatchString(v) {
		return v, nil
	}
	if lower := strings.ToLower(v); lower == "true" || lower == "false" {
		return lower, nil
	}
	if v == "null" {
		return "null", nil
	}
	return "'" + EscapeSOQLString(v) + "'", nil
}

// soqlListValue renders a comma-separated operator input as a SOQL IN-tuple.
// Each element goes through the same typing rules as a scalar, so
// "TODAY,YESTERDAY" and "a,b" both come out correctly.
func soqlListValue(v string) (string, error) {
	parts := strings.Split(v, ",")
	rendered := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lit, err := SOQLValue(p, false)
		if err != nil {
			return "", err
		}
		rendered = append(rendered, lit)
	}
	if len(rendered) == 0 {
		return "", fmt.Errorf("an IN comparison needs at least one value")
	}
	return "(" + strings.Join(rendered, ",") + ")", nil
}

// ---------------------------------------------------------------------------
// Field-type-aware value rendering
// ---------------------------------------------------------------------------
//
// Whether a SOQL literal must be quoted depends entirely on the FIELD's type,
// not the value's, and getting it wrong is a hard INVALID_FIELD error in both
// directions. Verified against a live org:
//
//	Amount > '10000'            -> INVALID_FIELD   (currency wants a bare number)
//	Amount > 10000              -> OK
//	Name = '12345'              -> OK              (text wants quotes)
//	Name = 12345                -> INVALID_FIELD
//	NumberOfEmployees > '10'    -> INVALID_FIELD
//	IsClosed = 'false'          -> INVALID_FIELD   (boolean wants a bare literal)
//	IsClosed = false            -> OK
//
// So no value-only heuristic can be correct. n8n quotes numeric-looking
// strings and tells the user to "pass a number via an expression" — which our
// operators cannot do, because every editor input is a string. That makes
// "opportunities over £10,000" unreachable, and it is one of the most obvious
// filters a sales admin would ever build.
//
// The fix is to ask Salesforce. DescribeObject already gives every field's
// type; FieldTypes caches that per object for the run so a get-many pays at
// most one extra call, and SOQLValueForType renders each value the way the
// field actually demands. When describe is unavailable (the connected user
// cannot see the object's metadata) it degrades to the value-only heuristic
// rather than failing the action.

// fieldTypeCache memoises describe-derived field types per (connection, object).
//
// The executor is a one-shot process — one flow run, then exit — so this
// cannot accrete across time the way a daemon's cache would. It exists so a
// Loop firing the same get-many a hundred times describes the object once.
//
// The key MUST include the connection, not just the object name. Two reasons,
// both reachable inside a single flow:
//
//   - A flow can hold two Salesforce nodes pointed at DIFFERENT orgs (a
//     sandbox-to-production sync is the obvious one). Custom fields differ
//     between orgs, and a field that is Currency in one can be Text in
//     another, so a shared entry renders the literal for the wrong org.
//   - describe output is filtered by the CONNECTED USER's field-level
//     security, so even two credentials against the same org legitimately see
//     different field sets.
//
// The api's describe cache is keyed the same way and for the same reason.
var fieldTypeCache = struct {
	mu sync.Mutex
	m  map[string]map[string]string
}{m: map[string]map[string]string{}}

// connectionKey fingerprints a connection for cache keying. The token is
// hashed, never stored — a cache key is not a place to keep a credential, and
// this map outlives individual calls within the run.
func connectionKey(instanceURL, token string) string {
	sum := sha256.Sum256([]byte(instanceURL + "\x00" + token))
	return hex.EncodeToString(sum[:16])
}

// FieldTypes returns a lower-cased field name -> Salesforce type map for an
// object, from its describe. Cached for the life of the process.
func FieldTypes(instanceURL, token, object string) (map[string]string, error) {
	obj, err := ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}
	key := connectionKey(instanceURL, token) + "|" + strings.ToLower(obj)

	fieldTypeCache.mu.Lock()
	if cached, ok := fieldTypeCache.m[key]; ok {
		fieldTypeCache.mu.Unlock()
		// Hand back a copy: the cached map is shared by every caller in the
		// run, so returning it directly lets one caller's mutation silently
		// rewrite another's view of the schema.
		return maps.Clone(cached), nil
	}
	fieldTypeCache.mu.Unlock()

	describe, err := DescribeObject(instanceURL, token, obj)
	if err != nil {
		return nil, err
	}
	types := map[string]string{}
	fields, _ := describe["fields"].([]interface{})
	for _, f := range fields {
		fm, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := fm["name"].(string)
		t, _ := fm["type"].(string)
		if name != "" && t != "" {
			types[strings.ToLower(name)] = strings.ToLower(t)
		}
	}

	fieldTypeCache.mu.Lock()
	fieldTypeCache.m[key] = types
	fieldTypeCache.mu.Unlock()
	return maps.Clone(types), nil
}

// numericSOQLTypes are the Salesforce field types whose literals must be bare.
var numericSOQLTypes = map[string]bool{
	"int": true, "double": true, "currency": true, "percent": true, "long": true,
}

// SOQLValueForType renders a value according to the FIELD's Salesforce type.
// An empty sfType means "unknown" and falls back to the value-only heuristic.
func SOQLValueForType(raw, sfType string, isList bool) (string, error) {
	v := strings.TrimSpace(raw)

	// A relative date keyword (TODAY, LAST_N_DAYS:7) is bare — but ONLY on a
	// field that actually holds a date. Checking it before the type meant a
	// TEXT field whose value happened to spell "today" was emitted as a bare
	// SOQL keyword, so the query matched on a date range instead of the word.
	if sfType == "" || sfType == "date" || sfType == "datetime" {
		upper := strings.ToUpper(v)
		if salesforceDateLiterals[upper] || relativeNDateLiteral.MatchString(upper) || agoNDateLiteral.MatchString(upper) {
			return upper, nil
		}
	}

	if isList {
		return soqlListValueForType(v, sfType)
	}

	switch {
	case sfType == "":
		return SOQLValue(v, false)

	case numericSOQLTypes[sfType]:
		if v == "" {
			return "null", nil
		}
		if _, err := strconv.ParseFloat(v, 64); err != nil {
			return "", fmt.Errorf("that field holds a number, so the comparison value must be a number — got %q", v)
		}
		return v, nil

	case sfType == "boolean":
		switch strings.ToLower(v) {
		case "true", "yes", "1":
			return "true", nil
		case "false", "no", "0":
			return "false", nil
		case "":
			return "null", nil
		}
		return "", fmt.Errorf("that field is a tick-box, so the comparison value must be true or false — got %q", v)

	case sfType == "date" || sfType == "datetime":
		if v == "" {
			return "null", nil
		}
		// Salesforce will not coerce between its Date and DateTime literal
		// forms, and rejects the mismatch outright. All three verified live:
		//
		//	CreatedDate >= 2026-07-01              INVALID_FIELD "must be of type dateTime"
		//	CreatedDate >= 2026-07-01T00:00:00Z    OK
		//	CloseDate   >  2026-07-25T00:00:00Z    INVALID_FIELD "must be of type date"
		//	CloseDate   >  2026-07-25              OK
		//	CreatedDate >= 2026-07-01T00:00:00     MALFORMED_QUERY (no offset)
		//
		// The operator cannot be expected to know which of two adjacent
		// dropdown entries is which — Contact alone carries ten date and
		// datetime fields side by side — so coerce rather than refuse. The
		// write path has always done this (SetDateIfPresent); the query path
		// simply never got it.
		return coerceDateLiteral(v, sfType)

	default:
		// string, picklist, id, reference, email, phone, url, textarea, ...
		if v == "" {
			return "''", nil
		}
		return "'" + EscapeSOQLString(v) + "'", nil
	}
}

// isoDateTimeNoOffset matches an ISO datetime with no timezone offset. Salesforce
// rejects that form outright — "2026-07-01T00:00:00" is MALFORMED_QUERY, not a
// value error — so it is normalised to UTC rather than refused. Silently
// shifting by the org's timezone instead would be worse than either.
var isoDateTimeNoOffset = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{1,3})?$`)

// coerceDateLiteral renders an operator's date value in the literal form the
// FIELD demands, converting between the two forms rather than rejecting a
// mismatch the operator has no way to anticipate.
func coerceDateLiteral(v, sfType string) (string, error) {
	switch {
	case isoDateTimeNoOffset.MatchString(v):
		// Tested BEFORE isoDateTimePattern: that pattern's offset group is
		// optional, so it also matches an offsetless value and would return it
		// unchanged — straight into MALFORMED_QUERY.
		if sfType == "date" {
			return v[:10], nil
		}
		return v + "Z", nil

	case isoDatePattern.MatchString(v):
		if sfType == "datetime" {
			// A Date field's value on a DateTime field: widen to midnight UTC.
			// Salesforce compares the instant, so this is the start of the day
			// the operator named.
			return v + "T00:00:00Z", nil
		}
		return v, nil

	case isoDateTimePattern.MatchString(v):
		if sfType == "date" {
			// A DateTime on a Date field: keep the calendar date. Same
			// truncation SetDateIfPresent applies on the write path.
			return v[:10], nil
		}
		return v, nil

	}
	return "", fmt.Errorf("that field holds a date, so the comparison value must be a date (2026-07-25), a date and time (2026-07-25T10:30:00Z) or a Salesforce date keyword such as TODAY or LAST_N_DAYS:7 — got %q", v)
}

func soqlListValueForType(v, sfType string) (string, error) {
	parts := strings.Split(v, ",")
	rendered := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		lit, err := SOQLValueForType(p, sfType, false)
		if err != nil {
			return "", err
		}
		rendered = append(rendered, lit)
	}
	if len(rendered) == 0 {
		return "", fmt.Errorf("an IN comparison needs at least one value")
	}
	return "(" + strings.Join(rendered, ",") + ")", nil
}

// Condition is one WHERE clause term from the operator's filter input.
type Condition struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// BuildWhere assembles a validated WHERE clause from filter conditions using
// the value-only heuristic. Prefer BuildWhereTyped, which asks Salesforce what
// each field actually is — see the field-type section above for why the
// heuristic cannot be right for numeric, boolean or date fields.
func BuildWhere(conditions []Condition, combineWithOr bool) (string, error) {
	return buildWhere(conditions, combineWithOr, nil)
}

// BuildWhereTyped assembles a validated WHERE clause, rendering each value
// according to its field's real Salesforce type. types is a lower-cased field
// name -> type map from FieldTypes; a nil map degrades to BuildWhere's
// heuristic, and an individual field missing from the map does too.
func BuildWhereTyped(conditions []Condition, combineWithOr bool, types map[string]string) (string, error) {
	return buildWhere(conditions, combineWithOr, types)
}

func buildWhere(conditions []Condition, combineWithOr bool, types map[string]string) (string, error) {
	if len(conditions) == 0 {
		return "", nil
	}
	terms := make([]string, 0, len(conditions))
	for _, c := range conditions {
		field, err := ValidateSOQLFieldName(c.Field)
		if err != nil {
			return "", err
		}
		op := c.Operator
		if strings.TrimSpace(op) == "" {
			op = "="
		}
		operator, err := ValidateSOQLOperator(op)
		if err != nil {
			return "", err
		}
		isList := operator == "IN" || operator == "NOT IN" || operator == "INCLUDES" || operator == "EXCLUDES"
		// Relationship fields (Account.Name) are typed on the far object, which
		// this map does not cover; those fall back to the heuristic, which is
		// correct for the text fields such traversals almost always target.
		sfType := ""
		if types != nil && !strings.Contains(field, ".") {
			sfType = types[strings.ToLower(field)]
		}
		value, err := SOQLValueForType(c.Value, sfType, isList)
		if err != nil {
			return "", fmt.Errorf("filter on %s: %w", field, err)
		}
		// SOQL has no binary NOT LIKE operator, and the negation must be a
		// SELF-CONTAINED group. Verified against a live org:
		//
		//	Name NOT LIKE 'x%'                  MALFORMED_QUERY
		//	NOT (Name LIKE 'x%')                OK   — but only ALONE
		//	NOT (Name LIKE 'x%') AND Employees>10   MALFORMED_QUERY
		//	Employees>10 AND NOT (Name LIKE 'x%')   MALFORMED_QUERY
		//	(NOT (Name LIKE 'x%'))              OK
		//	(NOT (Name LIKE 'x%')) AND Employees>10 OK
		//	Employees>10 AND (NOT (Name LIKE 'x%')) OK   (and with OR)
		//
		// The outer brackets are what make it composable: without them SOQL's
		// NOT binds the whole boolean expression, so the term only parses when
		// it is the entire WHERE clause. That is the trap — "does not contain"
		// is almost always scoped by something else, so the naive form works in
		// the one case nobody builds and fails in the ordinary one.
		//
		// n8n's node has the same whitelist and the same gap, so porting it
		// faithfully carried the bug across.
		if operator == "NOT LIKE" {
			terms = append(terms, "(NOT ("+field+" LIKE "+value+"))")
			continue
		}
		terms = append(terms, field+" "+operator+" "+value)
	}
	joiner := " AND "
	if combineWithOr {
		joiner = " OR "
	}
	return "WHERE " + strings.Join(terms, joiner), nil
}

// ParseConditions reads the filter-conditions input, which the editor supplies
// as a JSON array of {field, operator, value}. Returns nil when unset.
func ParseConditions(name string, inputs []*core.Connection) ([]Condition, error) {
	v, err := OptionalJSON(name, inputs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, nil
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a JSON array, e.g. [{"field":"Status","operator":"=","value":"Open"}]`, name)
	}
	out := make([]Condition, 0, len(arr))
	for i, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object with field, operator and value", name, i)
		}
		c := Condition{
			Field:    fmt.Sprintf("%v", firstNonNil(obj["field"], "")),
			Operator: fmt.Sprintf("%v", firstNonNil(obj["operator"], firstNonNil(obj["operation"], "="))),
			Value:    fmt.Sprintf("%v", firstNonNil(obj["value"], "")),
		}
		if strings.TrimSpace(c.Field) == "" {
			return nil, fmt.Errorf("%s[%d] is missing a field name", name, i)
		}
		out = append(out, c)
	}
	return out, nil
}

func firstNonNil(v, fallback interface{}) interface{} {
	if v == nil {
		return fallback
	}
	return v
}

// DefaultFields returns a sensible SELECT list for a standard object when the
// operator has not chosen fields. Ported from n8n's getDefaultFields so a
// get-many with no configuration returns something useful rather than just Id.
func DefaultFields(object string) string {
	switch strings.ToLower(object) {
	case "account":
		return "Id,Name,Type,LastModifiedDate"
	case "lead":
		return "Id,Company,FirstName,LastName,Street,PostalCode,City,Email,Status,LastModifiedDate"
	case "contact":
		return "Id,FirstName,LastName,Email,LastModifiedDate"
	case "opportunity":
		return "Id,AccountId,Amount,Probability,Type,StageName,CloseDate,LastModifiedDate"
	case "case":
		return "Id,AccountId,ContactId,Priority,Status,Subject,Type,LastModifiedDate"
	case "task":
		return "Id,Subject,Status,Priority,LastModifiedDate"
	case "event":
		return "Id,Subject,StartDateTime,EndDateTime,LastModifiedDate"
	case "campaign":
		return "Id,Name,Status,Type,StartDate,EndDate,LastModifiedDate"
	case "attachment":
		return "Id,Name,LastModifiedDate"
	case "user":
		return "Id,Name,Email,IsActive,LastModifiedDate"
	}
	return "Id,Name,LastModifiedDate"
}

// DefaultFieldsFor resolves a zero-configuration projection by ASKING the org,
// falling back to the static table only when describe is unavailable.
//
// The static fallback ends in "Id,Name,LastModifiedDate", and Name is the wrong
// guess on a large family of objects — verified live, every one of these is a
// hard INVALID_FIELD on "SELECT Id, Name":
//
//	CaseComment, ContentDocumentLink, OpportunityContactRole   no name field at all
//	Task, Event                                                Subject
//	Case                                                       CaseNumber
//	ContentDocument                                            Title
//
// That matters most for record_find and search_records, which are pointed at an
// arbitrary object by the operator, so the guess is wrong exactly when the
// action is doing its job. The describe is the one already cached by FieldTypes,
// so this costs nothing extra on a second call.
func DefaultFieldsFor(instanceURL, token, object string) string {
	obj, err := ValidateSOQLObjectName(object)
	if err != nil {
		return "Id"
	}
	// A curated list beats a derived one where we have it — it names the fields
	// an operator actually wants to see, not just the record's label.
	if known := DefaultFields(obj); known != "Id,Name,LastModifiedDate" {
		return known
	}
	describe, err := DescribeObject(instanceURL, token, obj)
	if err != nil {
		// Describe can be denied to a user who can still read records. "Id"
		// always works; a wrong guess does not.
		return "Id"
	}
	fields, _ := describe["fields"].([]interface{})
	nameField := ""
	hasModified := false
	for _, f := range fields {
		fm, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := fm["name"].(string)
		if isName, ok := fm["nameField"].(bool); ok && isName && nameField == "" {
			nameField = name
		}
		if name == "LastModifiedDate" {
			hasModified = true
		}
	}
	out := "Id"
	if nameField != "" && nameField != "Id" {
		out += "," + nameField
	}
	if hasModified {
		out += ",LastModifiedDate"
	}
	return out
}

// BuildSelect assembles a validated SELECT clause. fields is a comma-separated
// operator input; when blank the object's default field list is used.
func BuildSelect(object, fields string) (string, error) {
	obj, err := ValidateSOQLObjectName(object)
	if err != nil {
		return "", err
	}
	list := strings.TrimSpace(fields)
	if list == "" {
		list = DefaultFields(obj)
	}
	parts := strings.Split(list, ",")
	validated := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		// COUNT() and other aggregate forms are deliberately not accepted here;
		// they change the response shape and belong in the raw SOQL action.
		f, err := ValidateSOQLFieldName(p)
		if err != nil {
			return "", err
		}
		validated = append(validated, f)
	}
	if len(validated) == 0 {
		validated = append(validated, "Id")
	}
	return "SELECT " + strings.Join(validated, ", ") + " FROM " + obj, nil
}

// BuildQuery assembles a complete, validated SOQL statement for a get-many
// using the value-only heuristic. Prefer BuildQueryTyped.
func BuildQuery(object, fields string, conditions []Condition, combineWithOr bool, orderBy string, limit int, applyLimit bool) (string, error) {
	return buildQuery(object, fields, conditions, combineWithOr, orderBy, limit, applyLimit, nil)
}

// BuildQueryTyped assembles a get-many statement, resolving each filter value
// against its field's real Salesforce type via one cached describe call.
//
// A describe failure is NOT fatal: the connected user may lack metadata access
// on an object whose records they can still read perfectly well. In that case
// the query is built with the heuristic and the caller carries on — degrading
// to n8n's behaviour rather than refusing to run.
func BuildQueryTyped(instanceURL, token, object, fields string, conditions []Condition, combineWithOr bool, orderBy string, limit int, applyLimit bool) (string, error) {
	var types map[string]string
	if len(conditions) > 0 {
		if t, err := FieldTypes(instanceURL, token, object); err == nil {
			types = t
		}
	}
	return buildQuery(object, fields, conditions, combineWithOr, orderBy, limit, applyLimit, types)
}

func buildQuery(object, fields string, conditions []Condition, combineWithOr bool, orderBy string, limit int, applyLimit bool, types map[string]string) (string, error) {
	sel, err := BuildSelect(object, fields)
	if err != nil {
		return "", err
	}
	query := sel
	where, err := buildWhere(conditions, combineWithOr, types)
	if err != nil {
		return "", err
	}
	if where != "" {
		query += " " + where
	}
	if ob := strings.TrimSpace(orderBy); ob != "" {
		clause, err := BuildOrderBy(ob)
		if err != nil {
			return "", err
		}
		query += " " + clause
	}
	if applyLimit && limit > 0 {
		query += " LIMIT " + strconv.Itoa(limit)
	}
	return query, nil
}

// orderDirections is the closed set of sort directions permitted after a field.
var orderDirections = map[string]string{
	"ASC": "ASC", "DESC": "DESC",
	"ASC NULLS FIRST": "ASC NULLS FIRST", "ASC NULLS LAST": "ASC NULLS LAST",
	"DESC NULLS FIRST": "DESC NULLS FIRST", "DESC NULLS LAST": "DESC NULLS LAST",
}

// BuildOrderBy validates an "Field DESC, Other ASC" operator input into an
// ORDER BY clause. Both the identifiers and the directions are whitelisted —
// ORDER BY is as injectable as WHERE and is easy to forget.
func BuildOrderBy(raw string) (string, error) {
	parts := strings.Split(raw, ",")
	terms := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		words := strings.Fields(p)
		field, err := ValidateSOQLFieldName(words[0])
		if err != nil {
			return "", err
		}
		term := field
		if len(words) > 1 {
			dir := strings.ToUpper(strings.Join(words[1:], " "))
			canonical, ok := orderDirections[dir]
			if !ok {
				return "", fmt.Errorf("%q is not a valid sort direction — use ASC or DESC, optionally with NULLS FIRST or NULLS LAST", strings.Join(words[1:], " "))
			}
			term += " " + canonical
		}
		terms = append(terms, term)
	}
	if len(terms) == 0 {
		return "", nil
	}
	return "ORDER BY " + strings.Join(terms, ", "), nil
}

// ---------------------------------------------------------------------------
// Query execution
// ---------------------------------------------------------------------------

// queryResponse is Salesforce's SOQL result envelope.
type queryResponse struct {
	TotalSize      int                      `json:"totalSize"`
	Done           bool                     `json:"done"`
	NextRecordsURL string                   `json:"nextRecordsUrl"`
	Records        []map[string]interface{} `json:"records"`
}

// Query runs a SOQL statement and returns its records.
//
// When returnAll is false a single page is fetched and the outstanding
// nextRecordsUrl (if any) is returned so the caller can resume. When true the
// nextRecordsUrl chain is followed until Salesforce reports done, or the
// MaxAllPages cap is hit — a bound that exists so a query over a large org can
// never spin unbounded requests against an allowance shared with everything
// else the customer runs.
//
// includeDeleted selects /queryAll, which also returns records in the Recycle
// Bin and archived activities. There is no way to see deleted records without
// it, which is what makes it worth exposing.
func Query(instanceURL, token, soql string, returnAll, includeDeleted bool) (records []map[string]interface{}, nextURL string, totalSize int, pages int, err error) {
	records = []map[string]interface{}{}

	endpoint := "/query"
	if includeDeleted {
		endpoint = "/queryAll"
	}
	path := endpoint + "?q=" + url.QueryEscape(soql)

	absolute := false
	for {
		var resp *APIResponse
		if absolute {
			resp, err = ExecuteAbsolute(instanceURL, token, http.MethodGet, path, nil)
		} else {
			resp, err = ExecuteAPI(instanceURL, token, http.MethodGet, path, nil)
		}
		if err != nil {
			return nil, "", 0, pages, err
		}
		if err = CheckResponse(resp); err != nil {
			return nil, "", 0, pages, err
		}
		var qr queryResponse
		if err = json.Unmarshal(resp.Body, &qr); err != nil {
			return nil, "", 0, pages, fmt.Errorf("failed to parse Salesforce query response: %w", err)
		}
		pages++
		records = append(records, qr.Records...)
		totalSize = qr.TotalSize
		nextURL = qr.NextRecordsURL

		if qr.Done || nextURL == "" || !returnAll || pages >= MaxAllPages {
			break
		}
		// nextRecordsUrl is rooted at the instance and carries its own version.
		path = nextURL
		absolute = true
	}
	return records, nextURL, totalSize, pages, nil
}

// QueryOne runs a SOQL statement expected to match at most one record.
func QueryOne(instanceURL, token, soql string) (map[string]interface{}, error) {
	records, _, _, _, err := Query(instanceURL, token, soql, false, false)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, nil
	}
	return records[0], nil
}

// ---------------------------------------------------------------------------
// Record CRUD
// ---------------------------------------------------------------------------

// createResponse is Salesforce's create/upsert envelope.
type createResponse struct {
	ID      string    `json:"id"`
	Success bool      `json:"success"`
	Errors  []sfError `json:"errors"`
	Created *bool     `json:"created"` // upsert only: true=inserted, false=updated
}

// CreateRecord POSTs a new record of the given sObject type and returns the new
// record ID alongside the raw response.
func CreateRecord(instanceURL, token, object string, fields map[string]interface{}) (id string, raw map[string]interface{}, err error) {
	obj, err := ValidateSOQLObjectName(object)
	if err != nil {
		return "", nil, err
	}
	resp, err := ExecuteAPI(instanceURL, token, http.MethodPost, "/sobjects/"+obj, fields)
	if err != nil {
		return "", nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return "", nil, err
	}
	var cr createResponse
	if err := json.Unmarshal(resp.Body, &cr); err != nil {
		return "", nil, fmt.Errorf("failed to parse Salesforce create response: %w", err)
	}
	if !cr.Success && len(cr.Errors) > 0 {
		return "", nil, fmt.Errorf("Salesforce rejected the record: %s", formatSfErrorSlice(cr.Errors))
	}
	out, err := decode(resp)
	if err != nil {
		return "", nil, err
	}
	return cr.ID, out, nil
}

// GetRecord GETs a single record by ID. fields, when non-empty, restricts the
// response to those (validated) fields.
func GetRecord(instanceURL, token, object, id, fields string) (map[string]interface{}, error) {
	obj, err := ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}
	if err := ValidateRecordID(id); err != nil {
		return nil, err
	}
	path := "/sobjects/" + obj + "/" + url.PathEscape(id)
	if f := strings.TrimSpace(fields); f != "" {
		parts := strings.Split(f, ",")
		validated := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			v, err := ValidateSOQLFieldName(p)
			if err != nil {
				return nil, err
			}
			validated = append(validated, v)
		}
		if len(validated) > 0 {
			path += "?fields=" + url.QueryEscape(strings.Join(validated, ","))
		}
	}
	resp, err := ExecuteAPI(instanceURL, token, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// UpdateRecord PATCHes changes to a single record.
//
// Salesforce answers 204 No Content, so there is nothing to return but the ID
// the caller already supplied — which is exactly why it is returned rather than
// an empty map. Without it no downstream node can chain off an update.
func UpdateRecord(instanceURL, token, object, id string, fields map[string]interface{}) error {
	obj, err := ValidateSOQLObjectName(object)
	if err != nil {
		return err
	}
	if err := ValidateRecordID(id); err != nil {
		return err
	}
	if len(fields) == 0 {
		return fmt.Errorf("no fields to update — set at least one field")
	}
	resp, err := ExecuteAPI(instanceURL, token, http.MethodPatch, "/sobjects/"+obj+"/"+url.PathEscape(id), fields)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// DeleteRecord DELETEs a single record. Salesforce sends it to the Recycle Bin
// rather than destroying it, so this is recoverable for 15 days (or via
// record_undelete).
func DeleteRecord(instanceURL, token, object, id string) error {
	obj, err := ValidateSOQLObjectName(object)
	if err != nil {
		return err
	}
	if err := ValidateRecordID(id); err != nil {
		return err
	}
	resp, err := ExecuteAPI(instanceURL, token, http.MethodDelete, "/sobjects/"+obj+"/"+url.PathEscape(id), nil)
	if err != nil {
		return err
	}
	return CheckResponse(resp)
}

// UpsertRecord creates-or-updates a record matched on an external ID field.
//
// Two things the naive implementation gets wrong and this one does not:
// the external ID VALUE must be path-escaped (an email address as an external
// ID is the common case, and an unescaped "+" or "/" silently addresses the
// wrong record), and the match field must be REMOVED from the body — Salesforce
// rejects a payload that also sets the field it is matching on.
//
// Returns the record ID and whether it was created (true) or updated (false).
func UpsertRecord(instanceURL, token, object, externalIDField, externalIDValue string, fields map[string]interface{}) (id string, created bool, raw map[string]interface{}, err error) {
	obj, err := ValidateSOQLObjectName(object)
	if err != nil {
		return "", false, nil, err
	}
	extField, err := ValidateSOQLFieldName(externalIDField)
	if err != nil {
		return "", false, nil, err
	}
	if strings.TrimSpace(externalIDValue) == "" {
		return "", false, nil, fmt.Errorf("the external ID value is required — it is what Salesforce matches on")
	}
	body := make(map[string]interface{}, len(fields))
	for k, v := range fields {
		// Salesforce rejects a body that also sets the match field.
		if strings.EqualFold(k, extField) {
			continue
		}
		body[k] = v
	}
	path := "/sobjects/" + obj + "/" + extField + "/" + escapePathSegment(externalIDValue)
	resp, err := ExecuteAPI(instanceURL, token, http.MethodPatch, path, body)
	if err != nil {
		return "", false, nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return "", false, nil, err
	}
	out, err := decode(resp)
	if err != nil {
		return "", false, nil, err
	}
	// On v62: 201 with {id, success, created:true} when inserted, 200 with
	// {..., created:false} when matched — both carry a body. Older API versions
	// answered a matched upsert with a bare 204, which is why the decode below
	// is guarded rather than assumed.
	var cr createResponse
	if len(bytes.TrimSpace(resp.Body)) > 0 {
		_ = json.Unmarshal(resp.Body, &cr)
	}
	created = cr.Created != nil && *cr.Created
	return cr.ID, created, out, nil
}

// escapePathSegment escapes an operator-supplied value for a URL path segment.
//
// It is url.PathEscape plus an explicit "+" encoding. Strictly, "+" is a legal
// literal in a path and only means "space" in a query string, so PathEscape
// leaves it alone — but the most common external ID by far is an email
// address, plus-addressing is common in them, and anything along the way that
// treats the segment as form-encoded turns "a+b@x.com" into "a b@x.com" and
// upserts the WRONG record silently. n8n's encodeURIComponent escapes it for
// the same reason. The cost of being conservative here is nil; the cost of
// being right in theory and wrong in practice is a corrupted record.
func escapePathSegment(v string) string {
	return strings.ReplaceAll(url.PathEscape(v), "+", "%2B")
}

// recordIDPattern matches a Salesforce record ID: 15 (case-sensitive) or 18
// (case-safe) alphanumeric characters. Checking this locally turns a confusing
// server-side MALFORMED_ID into an immediate, specific message.
var recordIDPattern = regexp.MustCompile(`^[a-zA-Z0-9]{15}([a-zA-Z0-9]{3})?$`)

// ValidateRecordID confirms a value looks like a Salesforce record ID.
func ValidateRecordID(id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("a Salesforce record ID is required")
	}
	if !recordIDPattern.MatchString(id) {
		return fmt.Errorf("%q is not a Salesforce record ID — IDs are 15 or 18 letters and numbers, e.g. 00Q5f000004XyzAEAS", id)
	}
	return nil
}

func formatSfErrorSlice(errs []sfError) string {
	parts := make([]string, 0, len(errs))
	for _, e := range errs {
		msg := e.Message
		code := e.code()
		// A per-record Collections failure is addressed at one specific record,
		// so the 404-flavoured wording is the right branch for the codes that
		// differ by status (INVALID_CROSS_REFERENCE_KEY in particular).
		if explained := explainErrorCode(code, e.Fields, http.StatusNotFound); explained != "" {
			msg = explained + " (" + msg + ")"
		}
		if len(e.Fields) > 0 {
			msg += " — field(s): " + strings.Join(e.Fields, ", ")
		}
		// Carry the code, same reason as formatSalesforceErrors: callers branch
		// on it, and an operator reporting a failure needs something precise.
		if code != "" && !strings.Contains(msg, code) {
			msg += " [" + code + "]"
		}
		parts = append(parts, msg)
	}
	return strings.Join(parts, "; ")
}

// ---------------------------------------------------------------------------
// sObject Collections — bulk create / update / upsert / delete
// ---------------------------------------------------------------------------

// CollectionResult is one entry of a Collections response.
type CollectionResult struct {
	ID      string    `json:"id"`
	Success bool      `json:"success"`
	Errors  []sfError `json:"errors"`
}

// CollectionOutcome summarises a chunked bulk run.
type CollectionOutcome struct {
	Results   []map[string]interface{}
	SuccessNo int
	FailureNo int
	IDs       []string
	Failures  []string
}

// ChunkRecords splits a record slice into MaxCollectionRecords-sized chunks.
// Exposed so actions can report the chunk count in their summary.
func ChunkRecords(records []map[string]interface{}) [][]map[string]interface{} {
	var chunks [][]map[string]interface{}
	for i := 0; i < len(records); i += MaxCollectionRecords {
		end := i + MaxCollectionRecords
		if end > len(records) {
			end = len(records)
		}
		chunks = append(chunks, records[i:end])
	}
	return chunks
}

// CollectionWrite runs a chunked sObject Collections write.
//
// method is POST (create), PATCH (update/upsert). Each record is stamped with
// its attributes.type — Collections requires it per record even when every
// record is the same object.
//
// allOrNone is passed through per chunk, and that is a genuine limitation worth
// stating plainly: with 250 records and allOrNone true, a failure in the second
// chunk does NOT roll back the first, because they are separate transactions.
// Actions surface this in their field help rather than pretending otherwise.
//
// externalIDField, when non-empty, makes this an upsert keyed on that field.
func CollectionWrite(instanceURL, token, object, method string, records []map[string]interface{}, allOrNone bool, externalIDField string) (*CollectionOutcome, error) {
	obj, err := ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return nil, fmt.Errorf("no records supplied — pass an array of records to write")
	}

	path := "/composite/sobjects"
	if externalIDField != "" {
		extField, err := ValidateSOQLFieldName(externalIDField)
		if err != nil {
			return nil, err
		}
		path = "/composite/sobjects/" + obj + "/" + extField
	}

	outcome := &CollectionOutcome{Results: []map[string]interface{}{}, IDs: []string{}, Failures: []string{}}
	for chunkIdx, chunk := range ChunkRecords(records) {
		stamped := make([]map[string]interface{}, 0, len(chunk))
		for _, rec := range chunk {
			r := make(map[string]interface{}, len(rec)+1)
			for k, v := range rec {
				r[k] = v
			}
			r["attributes"] = map[string]interface{}{"type": obj}
			stamped = append(stamped, r)
		}
		payload := map[string]interface{}{"allOrNone": allOrNone, "records": stamped}

		resp, err := ExecuteAPI(instanceURL, token, method, path, payload)
		if err != nil {
			// Everything before this chunk is ALREADY COMMITTED in Salesforce.
			// Returning nil would tell the operator the whole run failed and
			// invite them to re-run it, duplicating every record written so
			// far. Hand back what actually landed, with the error.
			return outcome, err
		}
		if err := CheckResponse(resp); err != nil {
			return outcome, err
		}
		var results []CollectionResult
		if err := json.Unmarshal(resp.Body, &results); err != nil {
			return outcome, fmt.Errorf("failed to parse Salesforce collections response: %w", err)
		}
		for i, r := range results {
			recordIdx := chunkIdx*MaxCollectionRecords + i
			entry := map[string]interface{}{"index": recordIdx, "id": r.ID, "success": r.Success}
			if r.Success {
				outcome.SuccessNo++
				outcome.IDs = append(outcome.IDs, r.ID)
			} else {
				outcome.FailureNo++
				msg := formatSfErrorSlice(r.Errors)
				entry["error"] = msg
				outcome.Failures = append(outcome.Failures, fmt.Sprintf("record %d: %s", recordIdx, msg))
			}
			outcome.Results = append(outcome.Results, entry)
		}
	}
	return outcome, nil
}

// CollectionDelete deletes up to MaxCollectionRecords records per request,
// chunking automatically.
func CollectionDelete(instanceURL, token string, ids []string, allOrNone bool) (*CollectionOutcome, error) {
	if len(ids) == 0 {
		return nil, fmt.Errorf("no record IDs supplied")
	}
	for _, id := range ids {
		if err := ValidateRecordID(id); err != nil {
			return nil, err
		}
	}
	outcome := &CollectionOutcome{Results: []map[string]interface{}{}, IDs: []string{}, Failures: []string{}}
	for i := 0; i < len(ids); i += MaxCollectionRecords {
		end := i + MaxCollectionRecords
		if end > len(ids) {
			end = len(ids)
		}
		chunk := ids[i:end]
		q := url.Values{}
		q.Set("ids", strings.Join(chunk, ","))
		q.Set("allOrNone", strconv.FormatBool(allOrNone))
		resp, err := ExecuteAPI(instanceURL, token, http.MethodDelete, "/composite/sobjects?"+q.Encode(), nil)
		if err != nil {
			// As in CollectionWrite: earlier chunks are already deleted, so the
			// partial outcome has to survive the error or the operator cannot
			// tell what is left to do.
			return outcome, err
		}
		if err := CheckResponse(resp); err != nil {
			return outcome, err
		}
		var results []CollectionResult
		if err := json.Unmarshal(resp.Body, &results); err != nil {
			return outcome, fmt.Errorf("failed to parse Salesforce collections response: %w", err)
		}
		for j, r := range results {
			recordIdx := i + j
			entry := map[string]interface{}{"index": recordIdx, "id": r.ID, "success": r.Success}
			if r.Success {
				outcome.SuccessNo++
				outcome.IDs = append(outcome.IDs, r.ID)
			} else {
				outcome.FailureNo++
				msg := formatSfErrorSlice(r.Errors)
				entry["error"] = msg
				outcome.Failures = append(outcome.Failures, fmt.Sprintf("record %d: %s", recordIdx, msg))
			}
			outcome.Results = append(outcome.Results, entry)
		}
	}
	return outcome, nil
}

// ParseRecordArray reads a JSON-array input of records for the bulk actions.
func ParseRecordArray(name string, inputs []*core.Connection) ([]map[string]interface{}, error) {
	v, err := OptionalJSON(name, inputs)
	if err != nil {
		return nil, err
	}
	if v == nil {
		return nil, fmt.Errorf(`%s is required — a JSON array of records, e.g. [{"LastName":"Smith"},{"LastName":"Jones"}]`, name)
	}
	arr, ok := v.([]interface{})
	if !ok {
		return nil, fmt.Errorf(`%s must be a JSON array of records, e.g. [{"LastName":"Smith"}]`, name)
	}
	out := make([]map[string]interface{}, 0, len(arr))
	for i, item := range arr {
		obj, ok := item.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("%s[%d] must be an object of field names to values", name, i)
		}
		out = append(out, obj)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("%s is empty — pass at least one record", name)
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Describe
// ---------------------------------------------------------------------------

// DescribeObject fetches an sObject's full metadata (fields, picklist values,
// record types, child relationships).
//
// Worth knowing: describe is filtered by the CONNECTED user's permissions. A
// flow that works for the admin who built it can fail for the user whose token
// actually runs it, and the field simply will not be in the describe response
// rather than erroring.
func DescribeObject(instanceURL, token, object string) (map[string]interface{}, error) {
	obj, err := ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}
	resp, err := ExecuteAPI(instanceURL, token, http.MethodGet, "/sobjects/"+obj+"/describe", nil)
	if err != nil {
		return nil, err
	}
	if err := CheckResponse(resp); err != nil {
		return nil, err
	}
	return decode(resp)
}

// PicklistValues extracts the active values of a picklist field from a describe
// response. Returns nil when the field is absent or is not a picklist — the
// caller then falls back to free text rather than showing an empty dropdown.
func PicklistValues(describe map[string]interface{}, field string) []map[string]interface{} {
	fields, _ := describe["fields"].([]interface{})
	for _, f := range fields {
		fm, ok := f.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := fm["name"].(string)
		if !strings.EqualFold(name, field) {
			continue
		}
		raw, _ := fm["picklistValues"].([]interface{})
		out := make([]map[string]interface{}, 0, len(raw))
		for _, pv := range raw {
			pvm, ok := pv.(map[string]interface{})
			if !ok {
				continue
			}
			if active, ok := pvm["active"].(bool); ok && !active {
				continue
			}
			out = append(out, pvm)
		}
		return out
	}
	return nil
}

// ---------------------------------------------------------------------------
// SOAP bridge
// ---------------------------------------------------------------------------

// soapEnvelope is the Partner API request wrapper. A handful of operations —
// convertLead, merge, undelete — exist ONLY in the SOAP API; Salesforce has
// never shipped a REST equivalent. Rather than tell an operator that lead
// conversion is impossible (which is what every REST-only integration ends up
// doing), the node speaks just enough SOAP for those three, reusing the same
// OAuth access token as the <sessionId>.
const soapEnvelope = `<?xml version="1.0" encoding="utf-8"?>
<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/" xmlns:urn="urn:partner.soap.sforce.com">
  <soapenv:Header><urn:SessionHeader><urn:sessionId>%s</urn:sessionId></urn:SessionHeader></soapenv:Header>
  <soapenv:Body>%s</soapenv:Body>
</soapenv:Envelope>`

// XMLEscape escapes a value for inclusion in a SOAP body. Every operator-
// supplied value goes through this — the SOAP bodies are assembled as strings,
// so this is the same boundary EscapeSOQLString guards for queries.
func XMLEscape(s string) string {
	var b bytes.Buffer
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		// xml.EscapeText only fails if the writer fails; a bytes.Buffer cannot.
		return ""
	}
	return b.String()
}

// SOAPCall posts a Partner API request and returns the raw XML response body.
// innerXML is the operation element (e.g. "<urn:convertLead>...</urn:convertLead>").
func SOAPCall(instanceURL, token, innerXML string) ([]byte, error) {
	base := instanceURL
	if testBaseURL != "" {
		base = testBaseURL
	}
	endpoint := base + "/services/Soap/u/" + SOAPVersion

	body := fmt.Sprintf(soapEnvelope, XMLEscape(token), innerXML)
	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create SOAP request: %w", err)
	}
	req.Header.Set("Content-Type", "text/xml; charset=UTF-8")
	req.Header.Set("SOAPAction", `""`)

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Salesforce SOAP request failed: %w", redactError(err, token))
	}
	defer func() { _ = resp.Body.Close() }()
	// Same overflow detection as executeURL — a truncated SOAP envelope would
	// fail XML parsing with an equally opaque message.
	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBody+1))
	if err != nil {
		return nil, fmt.Errorf("failed to read SOAP response: %w", err)
	}
	if len(respBody) > maxResponseBody {
		return nil, fmt.Errorf("Salesforce sent back more data than this step can handle (over %d MB)", maxResponseBody>>20)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if fault := parseSOAPFault(respBody); fault != "" {
			return nil, fmt.Errorf("Salesforce error (%d): %s", resp.StatusCode, fault)
		}
		snippet := strings.TrimSpace(string(respBody))
		if len(snippet) > 500 {
			snippet = snippet[:500]
		}
		return nil, fmt.Errorf("Salesforce SOAP error (%d): %s", resp.StatusCode, snippet)
	}
	// A 200 can still carry a fault for some operations.
	if fault := parseSOAPFault(respBody); fault != "" {
		return nil, fmt.Errorf("Salesforce error: %s", fault)
	}
	return respBody, nil
}

// soapFault models the fault element of a SOAP response.
type soapFault struct {
	FaultCode   string `xml:"Body>Fault>faultcode"`
	FaultString string `xml:"Body>Fault>faultstring"`
}

// parseSOAPFault extracts a readable message from a SOAP fault, translating the
// exception code the same way explainErrorCode does for REST.
func parseSOAPFault(body []byte) string {
	var f soapFault
	if err := xml.Unmarshal(body, &f); err != nil {
		return ""
	}
	if f.FaultString == "" && f.FaultCode == "" {
		return ""
	}
	msg := strings.TrimSpace(f.FaultString)
	// faultcode looks like "sf:INVALID_SESSION_ID"; strip the namespace.
	code := f.FaultCode
	if i := strings.LastIndex(code, ":"); i >= 0 {
		code = code[i+1:]
	}
	if explained := explainErrorCode(code, nil, 0); explained != "" {
		if msg != "" {
			return explained + " (" + msg + ")"
		}
		return explained
	}
	if msg == "" {
		return code
	}
	return msg
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

// OptionalFloat extracts a numeric input as a float (Amount, Probability).
//
// core.Connection.Number() is int64-typed, so a Money/decimal value typed into
// the editor arrives as a string. Fall back to parsing the string form rather
// than silently truncating an Amount of 1250.50 to 1250.
func OptionalFloat(name string, inputs []*core.Connection) (float64, bool) {
	conn := core.FindConnection(name, inputs)
	if conn == nil {
		return 0, false
	}
	if s := conn.String(); s != nil {
		if v, err := strconv.ParseFloat(strings.TrimSpace(*s), 64); err == nil {
			return v, true
		}
	}
	if n := conn.Number(); n != nil {
		return float64(*n), true
	}
	return 0, false
}

// NumericInput reads an optional money or quantity input and distinguishes
// "left blank" from "typed something that is not a number".
//
// OptionalFloat cannot tell those apart — it answers (0, false) for both — so on
// its own an Amount of "£50,000" is indistinguishable from an Amount nobody
// filled in. The field is optional, so the value is dropped and Salesforce is
// told nothing about it: the run then reports success on a deal with no amount,
// and the operator's typo is never mentioned. Reading the raw string first makes
// the difference visible, so a value that was TYPED but unusable can be refused
// by name instead of silently discarded.
//
// example is the number quoted back in the error ("such as 499.00"), so the
// message suits the field — a line price, a quantity and a deal amount are not
// plausible in the same range.
//
// Returns (value, true, nil) when usable, (0, false, nil) when genuinely blank,
// and (0, false, err) when the operator typed something unusable.
func NumericInput(name, label, example string, inputs []*core.Connection) (float64, bool, error) {
	// OptionalString trims, so a whitespace-only value — a stray space, or a
	// variable that resolved to nothing — arrives here as "" and is treated as
	// blank rather than refused.
	raw := OptionalString(name, inputs)
	if raw == "" {
		return 0, false, nil
	}
	v, ok := OptionalFloat(name, inputs)
	if !ok {
		return 0, false, fmt.Errorf("%s must be a plain number such as %s — got %q. Leave out currency symbols, thousands separators and spaces", label, example, raw)
	}
	return v, true, nil
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
// Returns (nil, nil) when the input is absent/blank, (nil, err) on malformed
// JSON so the action can surface a clear message.
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

// SetIfPresent adds an optional string field to a record body only when the
// input was provided, so unset fields are omitted. Salesforce distinguishes an
// omitted field (leave alone) from an explicit null (clear it), and an update
// that sent every blank input would wipe half the record.
func SetIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v := OptionalString(inputName, inputs); v != "" {
		body[field] = v
	}
}

// SetIntIfPresent adds an optional integer field when its input is set.
func SetIntIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v, ok := OptionalInt(inputName, inputs); ok {
		body[field] = v
	}
}

// SetFloatIfPresent adds an optional numeric field when its input is set.
func SetFloatIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	if v, ok := OptionalFloat(inputName, inputs); ok {
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

// SetDateIfPresent adds an optional date field, trimming a datetime input down
// to the date-only form Salesforce demands for Date (not DateTime) fields such
// as Birthdate and CloseDate. Sending a full ISO timestamp to a Date field is
// rejected outright.
func SetDateIfPresent(body map[string]interface{}, inputs []*core.Connection, field, inputName string) {
	v := OptionalString(inputName, inputs)
	if v == "" {
		return
	}
	if i := strings.Index(v, "T"); i == 10 {
		v = v[:10]
	}
	body[field] = v
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
// onto the record body — the escape hatch for any Salesforce field not exposed
// as a first-class input, which for a platform where every org has custom
// fields is not an edge case but the normal way half of these actions get used.
// Later keys win. Returns an error on malformed JSON or the wrong shape.
func MergeAdditionalFields(body map[string]interface{}, inputs []*core.Connection) error {
	return MergeJSONObject(body, inputs, "additional_fields")
}

// MergeJSONObject overlays a named raw JSON object input onto a body.
func MergeJSONObject(body map[string]interface{}, inputs []*core.Connection, inputName string) error {
	v, err := OptionalJSON(inputName, inputs)
	if err != nil {
		return err
	}
	if v == nil {
		return nil
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return fmt.Errorf(`%s must be a JSON object, e.g. {"Custom_Field__c":"value"}`, inputName)
	}
	for k, val := range obj {
		body[k] = val
	}
	return nil
}

// ClampLimit bounds a requested page size to Salesforce's 1-2000 range, falling
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

// SplitList splits a comma-separated operator input into trimmed, non-empty
// values. Editor multi-value inputs are single-select, so multi-value fields
// are carried as comma-separated text.
func SplitList(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Result shaping
// ---------------------------------------------------------------------------

// ErrorResult is the standard soft-failure output map (returned with a nil
// error so the engine records it as data on the error port).
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"id":          "",
		"result":      map[string]interface{}{},
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// RecordResult shapes a single-record response into the standard action output.
//
// id is passed explicitly rather than read from the response because the most
// common Salesforce write — an update — answers 204 with no body at all. The
// caller always knows the ID; the response frequently does not.
func RecordResult(id string, record map[string]interface{}, summary string) map[string]interface{} {
	if record == nil {
		record = map[string]interface{}{}
	}
	if id == "" {
		id = StringifyID(record["Id"])
	}
	return map[string]interface{}{
		"id":          id,
		"result":      record,
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// ListResult shapes a query response into the standard list output.
func ListResult(records []map[string]interface{}, nextURL string, totalSize int, summary string) map[string]interface{} {
	// Non-nil so a zero-match list serialises as [] not null — get-many is
	// consumed by Loop nodes that iterate the array.
	items := make([]interface{}, 0, len(records))
	for _, r := range records {
		items = append(items, r)
	}
	return map[string]interface{}{
		"results":     items,
		"count":       len(items),
		"total_size":  totalSize,
		"next_url":    nextURL,
		"result":      map[string]interface{}{"records": items, "totalSize": totalSize, "done": nextURL == ""},
		"tool_result": summary,
		"success":     true,
		"error":       "",
	}
}

// TruncationHint warns that a list was capped, when nothing else can.
//
// Salesforce gives NO signal that an explicitly-LIMITed query left rows behind:
// "SELECT Id FROM Contact LIMIT 2" against 26 contacts answers totalSize:2,
// done:true and no nextRecordsUrl (verified live). So the only honest tell is
// that the page came back exactly full — and without it a truncated list reads
// as the complete answer, which is the worst way for a list to be wrong.
//
// Deliberately not a COUNT() round trip: that would double the API cost of the
// single most-used action in the node, and Flomation's explicit Loop node makes
// an operator's call volume the thing most likely to break their org.
func TruncationHint(returned, limit int, returnAll bool) string {
	if returnAll {
		return ""
	}
	// Clamp exactly as the QUERY did. Every caller passes the operator's raw
	// Limit input, but the query it built used ClampLimit — which substitutes
	// DefaultPageLimit when the input is blank. Comparing against the raw value
	// meant a blank Limit (0) short-circuited to "" and the hint never fired,
	// so the single most common configuration — leave Limit alone, more than
	// DefaultPageLimit records match — returned a capped list that read as the
	// complete answer. Which is the exact failure this function exists to stop.
	//
	// Clamping HERE rather than at the 19 call sites is deliberate: the two
	// values have to agree, and one place that cannot disagree with itself
	// beats nineteen that have to remember to.
	effective := ClampLimit(limit, limit > 0)
	if effective <= 0 || returned < effective {
		return ""
	}
	return fmt.Sprintf(" — this is the first %d; raise the Limit or turn on Return All if you need the rest", effective)
}

// BulkResult shapes a Collections outcome into the standard bulk output.
func BulkResult(outcome *CollectionOutcome, summary string) map[string]interface{} {
	results := make([]interface{}, 0, len(outcome.Results))
	for _, r := range outcome.Results {
		results = append(results, r)
	}
	ids := make([]interface{}, 0, len(outcome.IDs))
	for _, id := range outcome.IDs {
		ids = append(ids, id)
	}
	errText := ""
	if outcome.FailureNo > 0 {
		errText = strings.Join(outcome.Failures, "; ")
	}
	return map[string]interface{}{
		"results":       results,
		"ids":           ids,
		"success_count": outcome.SuccessNo,
		"failure_count": outcome.FailureNo,
		"result":        map[string]interface{}{"records": results},
		"tool_result":   summary,
		// A partial failure is still a successful call: the operator needs the
		// per-record detail, not a dead branch. Only a transport/auth failure
		// takes the error port.
		"success": true,
		"error":   errText,
	}
}

// PartialBulkResult shapes a bulk run that FAILED partway through.
//
// The chunks before the failure are already committed in Salesforce. Reporting
// only the error would tell the operator the whole run failed and invite them
// to re-run it, duplicating everything already written — the worst outcome
// available. This reports exactly what landed, so they can resume from it.
func PartialBulkResult(outcome *CollectionOutcome, err error, total int, object string) map[string]interface{} {
	if outcome == nil || outcome.SuccessNo == 0 {
		return ErrorResult(err.Error())
	}
	out := BulkResult(outcome, "")
	out["success"] = false
	out["error"] = err.Error()
	out["tool_result"] = fmt.Sprintf(
		"Stopped after writing %d of %d %s record(s) — those %d are already saved in Salesforce and will NOT be undone, so resume from record %d rather than re-running: %s",
		outcome.SuccessNo, total, object, outcome.SuccessNo, outcome.SuccessNo, err.Error())
	return out
}

// StringifyID renders an ID value as a clean string.
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

// SortedKeys returns a map's keys in a stable order, so summaries and error
// messages that enumerate fields read the same way on every run.
func SortedKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
