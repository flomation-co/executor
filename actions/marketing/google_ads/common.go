// Package google_ads_common holds the shared REST client, auth inputs, money
// helpers and result shapers for every Google Ads action. It has no Execute
// function, so the manifest generator excludes it from the action registry.
//
// The Google Ads API is unusual, and the shape of this package follows from it:
// there is essentially ONE read primitive (GoogleAdsService.Search, taking a
// GAQL query) and ONE write primitive (<resource>:mutate, taking a list of
// create/update/remove operations). Every list, report and "get" in this
// integration is a GAQL string; every create and update is an operation. So the
// per-action code is thin and almost all the real work — pagination, micros,
// row shaping and error flattening — lives here.
//
// Auth model: an OAuth2 managed credential (the Xero/QuickBooks model, not the
// paste-a-token model used by Meta Ads). A Google Ads access token lives for one
// hour, so a pasted token is dead before the flow it was pasted into has
// finished being built. Each action takes a `credential` input resolving to the
// current access token via ${credentials.X}; refresh is handled server-side by
// the API's refresh poller and never touched here.
//
// Deliberately NOT using the Google Ads client library: it is gRPC-based and
// would pull in a large dependency graph, which is how we previously caused a
// CI OOM. Hand-rolled REST, as with Apollo, Xero and Meta Ads.
//
// See PLAN-google-ads.md for the access-level path and the full action surface.
package google_ads_common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"

	core "flomation.app/automate/executor"
)

// BaseURL is the Google Ads API root. A var so tests can point it at an
// httptest server.
//
// Pinned to v25 (released 22 July 2026) rather than tracking "latest". Google
// sunsets a version roughly a year after release — v22 goes on 7 October 2026 —
// and a request to a sunset version fails outright with HTTP 400, so the
// version is a dated decision to revisit, not an incidental string.
var BaseURL = "https://googleads.googleapis.com/v25"

const (
	// Reports over a wide date range are genuinely slow; this is the same
	// budget the AI actions use.
	requestTimeout = 120 * time.Second

	// GAQL responses are large — a keyword report segmented by date runs to
	// tens of megabytes.
	maxResponse = 32 << 20

	// Guards, not features. A search page is a fixed 10,000 rows and each
	// response is capped at maxResponse, so without a bound a single
	// unfiltered query — SELECT ... FROM search_term_view DURING LAST_30_DAYS
	// on a large account will do it — could accumulate gigabytes in the
	// executor before anything downstream sees a row.
	//
	// Both bounds exist because they fail differently: the page cap stops a
	// query that keeps paginating, and the row cap stops one whose pages are
	// individually enormous. Either way the fix is the same and the message
	// says so: add a LIMIT, or narrow the date range.
	maxSearchPages = 20
	maxSearchRows  = 50000
)

// AuthInputs documents the credential trio every Google Ads action starts with.
//
// It is reference only: each action re-declares these inline, because the
// manifest generator resolves literal composite literals and silently emits an
// empty list for a cross-package var. That bit all 104 QuickBooks/Xero actions
// once (see CLAUDE.md) and it fails invisibly — Go builds fine, the editor just
// draws an input-less node.
var AuthInputs = []core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Google Ads Connection", Placeholder: "${credentials.MyGoogleAds}", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID (the ad account, e.g. 123-456-7890)", Required: true},
	{Name: "login_customer_id", Type: core.ConnectionTypeString, Label: "Manager Account ID (only when acting through an MCC)", Placeholder: "${credentials.MyGoogleAds.login_customer_id}"},
}

// StandardOutputs documents the common output contract. Reference only, for the
// same reason as AuthInputs.
var StandardOutputs = []core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// --- client ---

// Client is a Google Ads REST client scoped to one token. Built per call: the
// executor runs many tenants' flows concurrently, so a package-level client
// would leak one tenant's credentials into another's request. Same rule as
// stripe_common.NewClient and apollo_common.NewClient.
type Client struct {
	accessToken     string
	loginCustomerID string
	http            *http.Client
}

func NewClient(accessToken, loginCustomerID string) *Client {
	return &Client{
		accessToken:     accessToken,
		loginCustomerID: NormaliseCustomerID(loginCustomerID),
		http:            &http.Client{Timeout: requestTimeout},
	}
}

func reqContext(flow *core.Flow) context.Context {
	// Unit tests drive Execute with a nil flow.
	if flow == nil {
		return context.Background()
	}
	return flow.GoContext()
}

// Post sends a JSON body to a path below BaseURL and returns the decoded
// response.
//
// Note there is no developer-token header. Developer tokens were sunset on
// 9 September 2026; API access level now attaches to the Google Cloud project
// owning the OAuth client, and Google documents the header as "optional and
// ignored by the API servers" with rejection promised in a future major
// version. Sending one would be dead weight today and a failure later.
func (c *Client) Post(flow *core.Flow, path string, body interface{}) (map[string]interface{}, error) {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("could not encode the request: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}
	return c.do(flow, http.MethodPost, path, payload)
}

// Get performs a GET against a path below BaseURL.
func (c *Client) Get(flow *core.Flow, path string) (map[string]interface{}, error) {
	return c.do(flow, http.MethodGet, path, nil)
}

func (c *Client) do(flow *core.Flow, method, path string, body io.Reader) (map[string]interface{}, error) {
	req, err := http.NewRequestWithContext(reqContext(flow), method, BaseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.accessToken)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Required whenever the token belongs to a manager account acting on a
	// client account. Omitting it yields an authorisation error rather than a
	// "not found", which is why it reads as a broken integration rather than a
	// missing field.
	if c.loginCustomerID != "" {
		req.Header.Set("login-customer-id", c.loginCustomerID)
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request to the Google Ads API failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var decoded map[string]interface{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &decoded); err != nil {
			return nil, fmt.Errorf("unable to parse the Google Ads API response: %w", err)
		}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("%s", DescribeFailure(resp.StatusCode, decoded, raw))
	}
	return decoded, nil
}

// --- reading: GAQL ---

// Search runs a GAQL query and returns every row, following nextPageToken.
//
// pageSize is deliberately never sent: the API now returns
// PAGE_SIZE_NOT_SUPPORTED if it is set, and pages are a fixed 10,000 rows. Bound
// a result set with a LIMIT clause in the query instead.
func (c *Client) Search(flow *core.Flow, customerID, query string) ([]map[string]interface{}, error) {
	customerID = NormaliseCustomerID(customerID)
	if customerID == "" {
		return nil, fmt.Errorf("a customer ID is required")
	}

	var rows []map[string]interface{}
	token := ""
	for page := 0; page < maxSearchPages; page++ {
		body := map[string]interface{}{"query": query}
		if token != "" {
			body["pageToken"] = token
		}

		resp, err := c.Post(flow, "/customers/"+customerID+"/googleAds:search", body)
		if err != nil {
			return nil, err
		}

		results, _ := resp["results"].([]interface{})
		for _, r := range results {
			if m, ok := r.(map[string]interface{}); ok {
				rows = append(rows, m)
			}
		}
		if len(rows) > maxSearchRows {
			return rows, fmt.Errorf("the query returned more than %d rows — add a LIMIT clause, narrow the date range, or filter to one campaign", maxSearchRows)
		}

		token, _ = resp["nextPageToken"].(string)
		if token == "" {
			return rows, nil
		}
	}
	return rows, fmt.Errorf("the query returned more than %d pages (%d rows so far) — add a LIMIT clause or narrow the date range", maxSearchPages, len(rows))
}

// SearchFields queries googleAdsFields, the API's catalogue of every field:
// which are selectable, filterable and sortable, and which may appear together.
//
// This is what makes hand-written GAQL workable. The catalogue is account
// independent, so it takes no customer id, and it answers the question the
// query language cannot — "what can I ask for here?" — which a model otherwise
// has to guess at one failed query at a time.
func (c *Client) SearchFields(flow *core.Flow, query string) ([]map[string]interface{}, error) {
	var rows []map[string]interface{}
	token := ""
	for page := 0; page < maxSearchPages; page++ {
		body := map[string]interface{}{"query": query}
		if token != "" {
			body["pageToken"] = token
		}

		resp, err := c.Post(flow, "/googleAdsFields:search", body)
		if err != nil {
			return nil, err
		}

		results, _ := resp["results"].([]interface{})
		for _, r := range results {
			if m, ok := r.(map[string]interface{}); ok {
				rows = append(rows, m)
			}
		}

		token, _ = resp["nextPageToken"].(string)
		if token == "" {
			return rows, nil
		}
	}
	return rows, nil
}

// --- writing: mutate ---

// MutateOptions carries the two safety switches every mutating endpoint
// accepts.
type MutateOptions struct {
	// ValidateOnly asks the server to validate the operations fully and commit
	// nothing. Exposed on every mutating action as "Dry run", because these
	// actions spend real money and are driven by agents.
	ValidateOnly bool

	// PartialFailure commits the operations that succeed and returns errors for
	// the ones that do not, instead of failing the batch atomically. The right
	// default differs by action: true for bulk keyword adds, false for campaign
	// creation where a half-built campaign is worse than none.
	PartialFailure bool
}

// Mutate posts operations to a single resource's :mutate endpoint, e.g.
// resource "campaigns" for customers/{id}/campaigns:mutate.
func (c *Client) Mutate(flow *core.Flow, customerID, resource string, operations []interface{}, opt MutateOptions) (map[string]interface{}, error) {
	customerID = NormaliseCustomerID(customerID)
	if customerID == "" {
		return nil, fmt.Errorf("a customer ID is required")
	}
	if len(operations) == 0 {
		return nil, fmt.Errorf("no operations to perform")
	}

	resp, err := c.Post(flow, "/customers/"+customerID+"/"+resource+":mutate", map[string]interface{}{
		"operations":     operations,
		"validateOnly":   opt.ValidateOnly,
		"partialFailure": opt.PartialFailure,
	})
	if err != nil {
		return nil, err
	}
	if msg := partialFailure(resp); msg != "" {
		return resp, fmt.Errorf("%s", msg)
	}
	return resp, nil
}

// MutateAtomic posts a heterogeneous operation list to googleAds:mutate, which
// applies them across resource types in one transaction.
//
// This is what makes "create a campaign" a single call: a campaign references
// its budget by resource name, and the budget does not exist yet. Within one
// atomic mutate a NEGATIVE id is a temporary resource name — create the budget
// as customers/X/campaignBudgets/-1 and the campaign can point at it in the
// same request, with the server substituting the real id once everything
// validates. Nothing is committed unless all of it validates.
func (c *Client) MutateAtomic(flow *core.Flow, customerID string, operations []interface{}, opt MutateOptions) (map[string]interface{}, error) {
	customerID = NormaliseCustomerID(customerID)
	if customerID == "" {
		return nil, fmt.Errorf("a customer ID is required")
	}
	if len(operations) == 0 {
		return nil, fmt.Errorf("no operations to perform")
	}

	resp, err := c.Post(flow, "/customers/"+customerID+"/googleAds:mutate", map[string]interface{}{
		"mutateOperations": operations,
		"validateOnly":     opt.ValidateOnly,
		"partialFailure":   opt.PartialFailure,
	})
	if err != nil {
		return nil, err
	}
	if msg := partialFailure(resp); msg != "" {
		return resp, fmt.Errorf("%s", msg)
	}
	return resp, nil
}

// ListAccessibleCustomers returns the customer IDs the connected token can
// reach, as plain digit strings.
//
// This is the only Google Ads call that takes no customer id. It returns bare
// resource names and NO descriptive names, so it is never useful on its own —
// callers follow it with a GAQL query against `customer` or `customer_client`
// to get something a human can recognise.
func (c *Client) ListAccessibleCustomers(flow *core.Flow) ([]string, error) {
	resp, err := c.Get(flow, "/customers:listAccessibleCustomers")
	if err != nil {
		return nil, err
	}
	names, _ := resp["resourceNames"].([]interface{})
	out := make([]string, 0, len(names))
	for _, n := range names {
		if s, ok := n.(string); ok {
			out = append(out, strings.TrimPrefix(s, "customers/"))
		}
	}
	return out, nil
}

// MutatedResourceNames pulls the resource names out of a mutate response, in
// operation order. Empty entries are preserved so index N still corresponds to
// operation N.
func MutatedResourceNames(resp map[string]interface{}) []string {
	results, _ := resp["results"].([]interface{})
	// googleAds:mutate nests each result under a per-type key
	// (campaignResult, adGroupResult, …) rather than returning it flat.
	out := make([]string, 0, len(results))
	for _, r := range results {
		m, ok := r.(map[string]interface{})
		if !ok {
			out = append(out, "")
			continue
		}
		if name, ok := m["resourceName"].(string); ok {
			out = append(out, name)
			continue
		}
		nested := ""
		for _, v := range m {
			if inner, ok := v.(map[string]interface{}); ok {
				if name, ok := inner["resourceName"].(string); ok {
					nested = name
					break
				}
			}
		}
		out = append(out, nested)
	}
	return out
}

// ResourceID returns the trailing id of a resource name, e.g.
// "customers/123/campaigns/456" -> "456". Ad group criteria carry a compound
// trailing segment ("456~789") which is returned whole, because that compound
// form IS the criterion's identity.
func ResourceID(resourceName string) string {
	if resourceName == "" {
		return ""
	}
	parts := strings.Split(resourceName, "/")
	return parts[len(parts)-1]
}

// --- errors ---

// DescribeFailure turns Google's error envelope into something a flow author —
// or an agent reading tool_result — can act on.
//
// This matters more than any individual action. A failed mutate is an HTTP 400
// whose useful content is a GoogleAdsFailure buried in error.details[], holding
// one entry per failed operation with an error-code enum, a message, and a
// location naming the operation index and the offending field. Surfacing
// "400 Bad Request" turns a self-service fix into a support ticket; surfacing
// "operation 2: campaign.name — a campaign with this name already exists
// (DUPLICATE_CAMPAIGN_NAME)" does not.
func DescribeFailure(status int, decoded map[string]interface{}, raw []byte) string {
	envelope, _ := decoded["error"].(map[string]interface{})
	if envelope == nil {
		body := strings.TrimSpace(string(raw))
		if body == "" {
			body = "no response body"
		}
		return fmt.Sprintf("error from the Google Ads API (HTTP %d): %s", status, body)
	}

	summary, _ := envelope["message"].(string)
	if summary == "" {
		summary = fmt.Sprintf("HTTP %d", status)
	}

	details := describeFailureDetails(envelope["details"])
	if len(details) == 0 {
		// 401 means the access token is dead; the managed credential should have
		// refreshed it, so say which side to look at rather than "unauthorised".
		switch status {
		case 401:
			return fmt.Sprintf("the Google Ads credential was rejected (HTTP 401): %s — reconnect the Google Ads account in this environment", summary)
		case 403:
			return fmt.Sprintf("access denied by the Google Ads API (HTTP 403): %s — check the connected account can reach this customer ID, and that a Manager Account ID is set if you are acting through an MCC", summary)
		case 429:
			return fmt.Sprintf("rate limit or daily operation quota reached on the Google Ads API (HTTP 429): %s — see PLAN-google-ads.md for the access levels and their daily limits", summary)
		}
		return fmt.Sprintf("error from the Google Ads API (HTTP %d): %s", status, summary)
	}

	return fmt.Sprintf("Google Ads rejected the request (HTTP %d): %s\n  - %s",
		status, summary, strings.Join(details, "\n  - "))
}

// partialFailure renders the partialFailureError present on a mutate response
// when partial_failure was set and some operations failed. The successful ones
// have already been committed, so this is reported as an error on an otherwise
// 200 response.
func partialFailure(resp map[string]interface{}) string {
	envelope, _ := resp["partialFailureError"].(map[string]interface{})
	if envelope == nil {
		return ""
	}
	details := describeFailureDetails(envelope["details"])
	if len(details) == 0 {
		msg, _ := envelope["message"].(string)
		if msg == "" {
			return ""
		}
		return "some operations failed: " + msg
	}
	return fmt.Sprintf("%d operation(s) failed; the rest were applied:\n  - %s",
		len(details), strings.Join(details, "\n  - "))
}

// describeFailureDetails flattens every GoogleAdsFailure in a details array
// into one readable line per failed operation.
func describeFailureDetails(details interface{}) []string {
	list, _ := details.([]interface{})
	var out []string
	for _, d := range list {
		detail, ok := d.(map[string]interface{})
		if !ok {
			continue
		}
		errs, _ := detail["errors"].([]interface{})
		for _, e := range errs {
			entry, ok := e.(map[string]interface{})
			if !ok {
				continue
			}
			out = append(out, describeFailureEntry(entry))
		}
	}
	return out
}

func describeFailureEntry(entry map[string]interface{}) string {
	message, _ := entry["message"].(string)
	if message == "" {
		message = "no detail supplied"
	}

	var prefix string
	if op, path := failureLocation(entry); op != "" || path != "" {
		switch {
		case op != "" && path != "":
			prefix = op + ": " + path + " — "
		case op != "":
			prefix = op + ": "
		default:
			prefix = path + " — "
		}
	}

	suffix := ""
	if code := failureCode(entry); code != "" {
		suffix = " (" + code + ")"
	}
	return prefix + message + suffix
}

// failureCode reads the single-key errorCode object, e.g.
// {"campaignError": "DUPLICATE_CAMPAIGN_NAME"} -> "DUPLICATE_CAMPAIGN_NAME".
// The key names the error family and the value names the error; the value is
// the part worth showing, and the enum is what a user can search for.
func failureCode(entry map[string]interface{}) string {
	codes, _ := entry["errorCode"].(map[string]interface{})
	if len(codes) == 0 {
		return ""
	}
	// Map iteration order is random and these objects carry exactly one key in
	// practice; sorting keeps the output stable if that ever stops being true.
	keys := make([]string, 0, len(codes))
	for k := range codes {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if v, ok := codes[keys[0]].(string); ok {
		return v
	}
	return ""
}

// failureLocation turns location.fieldPathElements into an operation label and a
// field path. The first element is conventionally "operations" with an index,
// which becomes the label rather than part of the path, because "operation 2"
// is what the caller needs to map the error back to what it sent.
func failureLocation(entry map[string]interface{}) (operation, path string) {
	location, _ := entry["location"].(map[string]interface{})
	elements, _ := location["fieldPathElements"].([]interface{})

	var parts []string
	for i, e := range elements {
		element, ok := e.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := element["fieldName"].(string)
		if i == 0 && name == "operations" {
			if idx, ok := numeric(element["index"]); ok {
				operation = fmt.Sprintf("operation %d", int64(idx))
			}
			continue
		}
		if name == "" {
			continue
		}
		if idx, ok := numeric(element["index"]); ok {
			name = fmt.Sprintf("%s[%d]", name, int64(idx))
		}
		parts = append(parts, name)
	}
	return operation, strings.Join(parts, ".")
}

// --- customer ids ---

// NormaliseCustomerID strips everything that is not a digit.
//
// Google Ads shows customer IDs as 123-456-7890 everywhere in its own UI, and
// every header and URL wants 1234567890. Pasting straight out of the UI is the
// common case and the resulting failure is an unhelpful one, so this is applied
// to every id on the way in rather than being the caller's problem.
func NormaliseCustomerID(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// --- money: MICROS ---

// micros is the Google Ads money unit: 1,000,000 micros = one unit of the
// account's currency. £50.00 is 50000000.
const micros = 1000000

// MoneyToMicros converts a ConnectionTypeMoney input (a decimal in MAJOR units,
// e.g. "50.00") into the integer micros the API expects.
//
// NOT stripe_common.MoneyToMinorUnits, which meta_ads correctly uses because
// Meta budgets are in the currency's minor unit. Micros are a different scale
// entirely — reusing the Stripe helper here would be wrong by a factor of
// 10,000 for a two-decimal currency, i.e. a £50 daily budget submitted as half
// a penny, or the same error inverted on the way out.
//
// Micros are also currency-independent, so unlike minor units there is no
// 0-decimal (JPY) or 3-decimal (KWD) exponent table to get right, and no need
// to know the account's currency to do the conversion.
//
// Returns nil when the input is blank, so an unset amount stays unset rather
// than becoming zero — zero is a validation error at Google, but a silently
// omitted field is what "I did not set this" should mean.
func MoneyToMicros(name string, inputs []*core.Connection) (*int64, error) {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil, nil
	}
	// Tolerate a currency symbol or thousands separators pasted in alongside
	// the number; the editor's money field will not produce them, but a
	// ${...} substitution or an agent might.
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsDigit(r) || r == '.' || r == '-' {
			return r
		}
		return -1
	}, raw)

	rat, ok := new(big.Rat).SetString(cleaned)
	if !ok {
		return nil, fmt.Errorf("%s must be a decimal amount in major units (for example 50.00), not %q", name, raw)
	}
	if rat.Sign() < 0 {
		return nil, fmt.Errorf("%s cannot be negative", name)
	}

	// big.Rat rather than float64: 0.1 is not representable in binary floating
	// point, and a budget that is one micro out every time is the kind of
	// defect nobody finds until it is reconciled against an invoice.
	scaled := new(big.Rat).Mul(rat, new(big.Rat).SetInt64(micros))
	value := roundRat(scaled)
	return &value, nil
}

// roundRat rounds a non-negative rational to the nearest integer, half up.
func roundRat(r *big.Rat) int64 {
	num := new(big.Int).Set(r.Num())
	den := new(big.Int).Set(r.Denom())

	quo, rem := new(big.Int).QuoRem(num, den, new(big.Int))
	// remainder*2 >= denominator means the fraction is at least a half.
	if new(big.Int).Mul(rem, big.NewInt(2)).Cmp(den) >= 0 {
		quo.Add(quo, big.NewInt(1))
	}
	return quo.Int64()
}

// MicrosToMajor renders a micros value as a plain decimal in major units.
//
// The read side matters as much as the write side: metrics.cost_micros comes
// back as a string of micros, and an action that passes it through unconverted
// has an agent reporting "spend was 5,430,000" when it was 5.43.
func MicrosToMajor(value int64) string {
	negative := value < 0
	if negative {
		value = -value
	}
	whole := value / micros
	frac := value % micros

	out := strconv.FormatInt(whole, 10)
	if frac != 0 {
		// Trim trailing zeros so 5430000 reads as "5.43" rather than "5.430000".
		fraction := strings.TrimRight(fmt.Sprintf("%06d", frac), "0")
		out += "." + fraction
	}
	if negative {
		out = "-" + out
	}
	return out
}

// --- row shaping ---

// FlattenRow turns a nested GAQL result row into flat, dotted keys that match
// the field names the caller SELECTed.
//
// Two conversions happen here, and both exist to close a gap that would
// otherwise surprise every caller:
//
//  1. GAQL is written in snake_case (metrics.cost_micros) but the REST response
//     comes back in lowerCamelCase (metrics.costMicros). Selecting one name and
//     receiving another is a trap for a human and an outright failure for an
//     agent that assumes what it asked for is what it got, so keys are converted
//     back to the snake_case form the query used.
//
//  2. Any *_micros value gains a sibling without the suffix holding the major
//     unit amount — metrics.cost_micros "5430000" also yields metrics.cost
//     "5.43". Additive, so the raw value is still there for anything that wants
//     it.
func FlattenRow(row map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	flattenInto(out, "", row)
	return out
}

func flattenInto(out map[string]interface{}, prefix string, value map[string]interface{}) {
	for key, raw := range value {
		path := SnakeCase(key)
		if prefix != "" {
			path = prefix + "." + path
		}

		switch v := raw.(type) {
		case map[string]interface{}:
			flattenInto(out, path, v)
		default:
			out[path] = raw
			if major, ok := majorFromMicros(path, raw); ok {
				out[strings.TrimSuffix(path, "_micros")] = major
			}
		}
	}
}

// majorFromMicros converts a *_micros field to its major-unit decimal. Values
// arrive as JSON strings because the API maps int64 to string, so both forms
// are handled.
func majorFromMicros(path string, raw interface{}) (string, bool) {
	if !strings.HasSuffix(path, "_micros") {
		return "", false
	}
	switch v := raw.(type) {
	case string:
		parsed, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return "", false
		}
		return MicrosToMajor(parsed), true
	case float64:
		return MicrosToMajor(int64(v)), true
	}
	return "", false
}

// FlattenRows applies FlattenRow across a result set.
func FlattenRows(rows []map[string]interface{}) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		out = append(out, FlattenRow(row))
	}
	return out
}

// SnakeCase converts a lowerCamelCase JSON key to the snake_case form GAQL
// uses. Digits are treated as part of the preceding word, so "type2" stays
// "type2" rather than becoming "type_2".
func SnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if unicode.IsUpper(r) {
			if i > 0 {
				b.WriteByte('_')
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// --- input helpers ---

// GetAuth extracts the resolved access token, customer id and optional manager
// id, refusing an unresolved ${...} reference rather than sending it as a
// literal bearer token and getting an opaque 401 back.
func GetAuth(inputs []*core.Connection) (token, customerID, loginCustomerID string, err error) {
	token, err = RequiredString("credential", inputs)
	if err != nil {
		return "", "", "", fmt.Errorf("connect a Google Ads account: %w", err)
	}
	if strings.HasPrefix(token, "${") {
		return "", "", "", fmt.Errorf("the Google Ads credential did not resolve — connect and authorise a Google Ads account in this environment")
	}

	raw, err := RequiredString("customer_id", inputs)
	if err != nil {
		return "", "", "", fmt.Errorf("a customer ID is required — this is the ad account, shown as 123-456-7890 in Google Ads")
	}
	customerID = NormaliseCustomerID(raw)
	if customerID == "" {
		return "", "", "", fmt.Errorf("%q is not a valid customer ID — it should be the ten-digit account number, with or without dashes", raw)
	}

	login := OptionalString("login_customer_id", inputs)
	if strings.HasPrefix(login, "${") {
		// An unresolved optional reference means "no manager account", not a
		// literal header value.
		login = ""
	}
	return token, customerID, NormaliseCustomerID(login), nil
}

func RequiredString(name string, inputs []*core.Connection) (string, error) {
	v := OptionalString(name, inputs)
	if v == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return v, nil
}

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	v := strings.TrimSpace(*c.String())
	// An unresolved reference is not a value. Passing "${flow.thing}" through
	// to GAQL produces a syntax error that names the query rather than the
	// missing variable.
	if strings.HasPrefix(v, "${") {
		return ""
	}
	return v
}

func OptionalBool(name string, inputs []*core.Connection) bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return false
	}
	if b := c.Boolean(); b != nil {
		return *b
	}
	// The editor stores some boolean values as strings.
	if s := c.String(); s != nil {
		return strings.EqualFold(strings.TrimSpace(*s), "true")
	}
	return false
}

// OptionalList reads a multi-select input.
//
// The editor stores one of these as either a JSON array or a comma-separated
// string depending on how it was set (a picked value versus a ${...}
// substitution), so both are accepted — the same tolerance the Acuity and AWX
// triggers need for the same reason.
func OptionalList(name string, inputs []*core.Connection) []string {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}

	if strings.HasPrefix(raw, "[") {
		var decoded []string
		if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
			return trimAll(decoded)
		}
	}
	return trimAll(strings.Split(raw, ","))
}

func trimAll(values []string) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

func OptionalInt(name string, inputs []*core.Connection) *int64 {
	raw := OptionalString(name, inputs)
	if raw == "" {
		return nil
	}
	v, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

// MutateOptionsFrom reads the two standard safety inputs. defaultPartial is the
// action's own choice, overridden only when the author sets the input.
func MutateOptionsFrom(inputs []*core.Connection, defaultPartial bool) MutateOptions {
	opt := MutateOptions{
		ValidateOnly:   OptionalBool("validate_only", inputs),
		PartialFailure: defaultPartial,
	}
	if c := core.FindConnection("partial_failure", inputs); c != nil {
		if b := c.Boolean(); b != nil {
			opt.PartialFailure = *b
		} else if s := c.String(); s != nil && strings.TrimSpace(*s) != "" && !strings.HasPrefix(strings.TrimSpace(*s), "${") {
			opt.PartialFailure = strings.EqualFold(strings.TrimSpace(*s), "true")
		}
	}
	return opt
}

// --- GAQL construction ---

// QuoteGAQL renders a string as a GAQL literal, escaping the quote and escape
// characters.
//
// GAQL values cannot be bound as parameters — there is no placeholder syntax —
// so any value interpolated into a query has to be escaped here. Treat this the
// same way the SQL actions treat identifier quoting: it is the only thing
// standing between a campaign named with an apostrophe and a malformed query,
// and between a hostile value and a rewritten WHERE clause.
func QuoteGAQL(value string) string {
	escaped := strings.ReplaceAll(value, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `'`, `\'`)
	// Newlines would terminate nothing but do make the resulting error
	// unreadable; GAQL has no multi-line literal.
	escaped = strings.ReplaceAll(escaped, "\n", " ")
	escaped = strings.ReplaceAll(escaped, "\r", " ")
	return "'" + escaped + "'"
}

// DateRangeClause turns a date-range selection into a GAQL condition on
// segments.date.
//
// Google's named ranges (LAST_30_DAYS and friends) are unquoted keywords after
// DURING, whereas explicit dates are quoted and use BETWEEN. Getting that the
// wrong way round is a syntax error rather than a wrong answer, which at least
// fails loudly.
//
// Dates are interpreted in the ACCOUNT's timezone, not the flow's and not UTC.
func DateRangeClause(preset, from, to string) (string, error) {
	preset = strings.TrimSpace(strings.ToUpper(preset))
	if preset != "" && preset != "CUSTOM" {
		if !validDatePreset(preset) {
			return "", fmt.Errorf("%q is not a recognised date range", preset)
		}
		return "segments.date DURING " + preset, nil
	}
	if from == "" || to == "" {
		if preset == "CUSTOM" {
			return "", fmt.Errorf("a custom date range needs both a start and an end date (YYYY-MM-DD)")
		}
		return "", nil
	}
	if err := validDate(from); err != nil {
		return "", fmt.Errorf("start date: %w", err)
	}
	if err := validDate(to); err != nil {
		return "", fmt.Errorf("end date: %w", err)
	}
	return fmt.Sprintf("segments.date BETWEEN %s AND %s", QuoteGAQL(from), QuoteGAQL(to)), nil
}

// DatePresets are the named ranges GAQL accepts after DURING. Exposed so the
// reporting actions can offer them as a dropdown without restating the list.
var DatePresets = []string{
	"TODAY", "YESTERDAY", "LAST_7_DAYS", "LAST_14_DAYS", "LAST_30_DAYS",
	"THIS_WEEK_SUN_TODAY", "THIS_WEEK_MON_TODAY", "LAST_WEEK_SUN_SAT", "LAST_WEEK_MON_SUN",
	"THIS_MONTH", "LAST_MONTH", "LAST_BUSINESS_WEEK", "THIS_QUARTER", "LAST_QUARTER",
	"THIS_YEAR", "LAST_YEAR", "LAST_90_DAYS",
}

func validDatePreset(preset string) bool {
	for _, p := range DatePresets {
		if p == preset {
			return true
		}
	}
	return false
}

func validDate(value string) error {
	if _, err := time.Parse("2006-01-02", value); err != nil {
		return fmt.Errorf("%q is not a date in YYYY-MM-DD form", value)
	}
	return nil
}

// CampaignDateTime turns a plain YYYY-MM-DD date into the timestamp a campaign's
// startDateTime / endDateTime wants.
//
// Worth knowing, because it changed: v25 campaigns carry start_date_time and
// end_date_time in "yyyy-MM-dd HH:mm:ss" form, replacing the older
// start_date / end_date (which were bare YYYYMMDD). Code written against the
// older shape fails with an unhelpful field error rather than a date error.
//
// The time component is set to the edges of the day — 00:00:00 to start,
// 23:59:59 to end — which is the daily granularity every campaign type
// supports. Timestamps are interpreted in the SERVING CUSTOMER's timezone, not
// the flow's and not UTC, so a campaign asked to end "today" ends at the
// account's midnight.
func CampaignDateTime(date string, endOfDay bool) (string, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return "", nil
	}
	// Accept an already-complete timestamp unchanged, so an author who knows
	// they want a mid-day boundary is not overruled.
	if len(date) > len("2006-01-02") {
		if _, err := time.Parse("2006-01-02 15:04:05", date); err == nil {
			return date, nil
		}
		return "", fmt.Errorf("%q is not a date in YYYY-MM-DD form (or a timestamp as YYYY-MM-DD HH:MM:SS)", date)
	}
	if err := validDate(date); err != nil {
		return "", err
	}
	if endOfDay {
		return date + " 23:59:59", nil
	}
	return date + " 00:00:00", nil
}

// BuildQuery assembles a GAQL statement from its parts. Empty conditions are
// dropped, so a caller can build a condition list without guarding each one.
func BuildQuery(selectFields []string, resource string, conditions []string, orderBy string, limit *int64) string {
	var b strings.Builder
	b.WriteString("SELECT ")
	b.WriteString(strings.Join(selectFields, ", "))
	b.WriteString(" FROM ")
	b.WriteString(resource)

	var kept []string
	for _, c := range conditions {
		if strings.TrimSpace(c) != "" {
			kept = append(kept, c)
		}
	}
	if len(kept) > 0 {
		b.WriteString(" WHERE ")
		b.WriteString(strings.Join(kept, " AND "))
	}
	if strings.TrimSpace(orderBy) != "" {
		b.WriteString(" ORDER BY ")
		b.WriteString(orderBy)
	}
	if limit != nil && *limit > 0 {
		b.WriteString(" LIMIT ")
		b.WriteString(strconv.FormatInt(*limit, 10))
	}
	return b.String()
}

// --- keywords ---

// Google's limits on a single keyword.
const (
	maxKeywordLength = 80
	maxKeywordWords  = 10
	maxKeywordBatch  = 1000
)

// KeywordTerms parses a one-per-line keyword input into clean terms.
//
// Two things are handled that would otherwise cost someone an afternoon:
//
//   - Match-type punctuation is stripped. Google Ads writes phrase match as
//     "term" and exact match as [term], and both the editor UI and exported
//     reports use that notation, so it is what people paste. Here the match type
//     is a separate input, and leaving the punctuation in place would create a
//     keyword whose literal text contains brackets — which matches nothing and
//     looks, in the UI, exactly like the keyword they wanted.
//   - Length and word count are checked locally, because the server-side
//     rejection identifies the operation by index rather than by the offending
//     keyword text.
func KeywordTerms(raw string) ([]string, error) {
	var terms []string
	seen := map[string]bool{}

	for _, line := range strings.Split(raw, "\n") {
		term := strings.TrimSpace(line)
		if term == "" {
			continue
		}

		if len(term) >= 2 {
			first, last := term[0], term[len(term)-1]
			if (first == '[' && last == ']') || (first == '"' && last == '"') {
				term = strings.TrimSpace(term[1 : len(term)-1])
			}
		}
		if term == "" {
			continue
		}

		// Google treats keywords case-insensitively, so two lines differing only
		// in case are one keyword and the second would be rejected as a
		// duplicate. Dropping it here is quieter and costs no operation.
		key := strings.ToLower(term)
		if seen[key] {
			continue
		}
		seen[key] = true

		if length := len([]rune(term)); length > maxKeywordLength {
			return nil, fmt.Errorf("the keyword %q is %d characters, over Google's %d character limit", term, length, maxKeywordLength)
		}
		if words := len(strings.Fields(term)); words > maxKeywordWords {
			return nil, fmt.Errorf("the keyword %q has %d words, over Google's %d word limit", term, words, maxKeywordWords)
		}
		terms = append(terms, term)
	}

	if len(terms) == 0 {
		return nil, fmt.Errorf("no keywords were given — enter one per line")
	}
	if len(terms) > maxKeywordBatch {
		return nil, fmt.Errorf("%d keywords is more than the %d this action sends in one request — split them across several runs", len(terms), maxKeywordBatch)
	}
	return terms, nil
}

// --- bidding ---

// ApplyBiddingStrategy sets the chosen bidding strategy on a campaign payload.
//
// A campaign carries its strategy as a NESTED OBJECT under a
// strategy-specific key (manualCpc, targetSpend, targetCpa …) rather than as an
// enum field — biddingStrategyType is output-only, computed from whichever of
// those objects is present. Setting the enum instead of the object is the
// natural mistake and produces a campaign with no strategy at all.
//
// Returns the update-mask paths that were touched, so an update can declare
// exactly what it changed.
func ApplyBiddingStrategy(strategy string, inputs []*core.Connection, campaign map[string]interface{}) ([]string, error) {
	strategy = strings.TrimSpace(strings.ToUpper(strategy))
	if strategy == "" {
		return nil, nil
	}

	targetCPA, err := MoneyToMicros("target_cpa", inputs)
	if err != nil {
		return nil, err
	}
	targetROAS := OptionalString("target_roas", inputs)

	switch strategy {
	case "MANUAL_CPC":
		campaign["manualCpc"] = map[string]interface{}{"enhancedCpcEnabled": OptionalBool("enhanced_cpc", inputs)}
		return []string{"manual_cpc"}, nil

	case "TARGET_SPEND", "MAXIMIZE_CLICKS":
		// Google's own UI calls this "Maximise clicks"; the API calls it
		// TargetSpend. Both spellings are accepted so a flow author can use
		// whichever name they know.
		campaign["targetSpend"] = map[string]interface{}{}
		return []string{"target_spend"}, nil

	case "MAXIMIZE_CONVERSIONS":
		payload := map[string]interface{}{}
		if targetCPA != nil {
			payload["targetCpaMicros"] = strconv.FormatInt(*targetCPA, 10)
		}
		campaign["maximizeConversions"] = payload
		return []string{"maximize_conversions"}, nil

	case "MAXIMIZE_CONVERSION_VALUE":
		payload := map[string]interface{}{}
		if targetROAS != "" {
			roas, err := strconv.ParseFloat(targetROAS, 64)
			if err != nil {
				return nil, fmt.Errorf("target ROAS must be a number, for example 4 for a 400%% return")
			}
			payload["targetRoas"] = roas
		}
		campaign["maximizeConversionValue"] = payload
		return []string{"maximize_conversion_value"}, nil

	case "TARGET_CPA":
		if targetCPA == nil {
			return nil, fmt.Errorf("Target CPA bidding needs a target CPA amount")
		}
		campaign["targetCpa"] = map[string]interface{}{"targetCpaMicros": strconv.FormatInt(*targetCPA, 10)}
		return []string{"target_cpa"}, nil

	case "TARGET_ROAS":
		if targetROAS == "" {
			return nil, fmt.Errorf("Target ROAS bidding needs a target ROAS, for example 4 for a 400%% return")
		}
		roas, err := strconv.ParseFloat(targetROAS, 64)
		if err != nil {
			return nil, fmt.Errorf("target ROAS must be a number, for example 4 for a 400%% return")
		}
		campaign["targetRoas"] = map[string]interface{}{"targetRoas": roas}
		return []string{"target_roas"}, nil
	}
	return nil, fmt.Errorf("%q is not a bidding strategy this action can set", strategy)
}

// --- result shapers ---

func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{"tool_result": "Error: " + msg, "success": false, "error": msg}
}

func OkResult(summary string, extra map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{"tool_result": summary, "success": true, "error": ""}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// RowsResult is the standard shape for anything that runs a GAQL query.
//
// Both forms of the data are returned deliberately: `rows` is the API's nested
// JSON, faithful and suitable for Object Get Field downstream, while `table` is
// flattened to the dotted snake_case names the query used, with micros
// converted. tool_result embeds the flattened table rather than a count,
// because an agent given "Found 12 campaigns" and nothing else has to make a
// second call to do anything useful (see executor!305).
func RowsResult(rows []map[string]interface{}, summary string, extra map[string]interface{}) map[string]interface{} {
	if rows == nil {
		rows = []map[string]interface{}{}
	}
	table := FlattenRows(rows)

	body := summary
	if encoded, err := json.Marshal(table); err == nil && len(encoded) > 2 {
		body = summary + ":\n" + string(encoded)
	}

	out := map[string]interface{}{
		"tool_result": body,
		"rows":        rows,
		"table":       table,
		"count":       len(rows),
		"success":     true,
		"error":       "",
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

// MutateResult is the standard shape for anything that writes.
//
// A dry run is reported as a success with dry_run true and an explicit note in
// tool_result, because "Validated ..." and "Created ..." must never be
// mistakable for each other by a person skim-reading an execution or by an
// agent deciding whether to move on.
func MutateResult(resp map[string]interface{}, opt MutateOptions, summary string, extra map[string]interface{}) map[string]interface{} {
	names := MutatedResourceNames(resp)

	text := summary
	if opt.ValidateOnly {
		text = "Dry run — Google Ads validated this successfully but NOTHING was applied. Intended change: " + summary
	} else if len(names) > 0 && names[0] != "" {
		text = summary + " (" + strings.Join(names, ", ") + ")"
	}

	out := map[string]interface{}{
		"tool_result":    text,
		"resource_names": names,
		"count":          len(names),
		"dry_run":        opt.ValidateOnly,
		"success":        true,
		"error":          "",
	}
	if len(names) > 0 {
		out["resource_name"] = names[0]
		out["id"] = ResourceID(names[0])
	}
	for k, v := range extra {
		out[k] = v
	}
	return out
}

func numeric(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int64:
		return float64(n), true
	case string:
		f, err := strconv.ParseFloat(n, 64)
		return f, err == nil
	}
	return 0, false
}
