// Package apollo_common holds the shared REST client, auth input, input helpers
// and result shapers for every Apollo.io action. It has no Execute function, so
// the manifest generator excludes it from the action registry.
//
// Auth model: Apollo is a paste-an-API-key integration (single key per
// workspace). Each action takes an `api_key` input of ConnectionTypeSecret,
// resolved from an environment secret (${secrets.X}). The key is sent as the
// X-Api-Key header. CRITICAL: the executor runs many tenants' flows
// concurrently, so the key must be bound to a per-call client — never a
// package-level global — to avoid cross-tenant leakage.
package apollo_common

import (
	"bytes"
	"context"
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

// BaseURL is the Apollo API root. A var so tests can point it at an httptest
// server. Apollo's current REST surface lives under /api/v1.
var BaseURL = "https://api.apollo.io/api/v1"

const requestTimeout = 60 * time.Second

// reqContext returns the flow's cancellation context, or a background context
// when flow is nil (unit tests drive Execute with a nil flow).
func reqContext(flow *core.Flow) context.Context {
	if flow == nil {
		return context.Background()
	}
	return flow.GoContext()
}

// Client is an Apollo REST client scoped to a single tenant's API key.
type Client struct {
	apiKey string
	http   *http.Client
}

// NewClient builds a client bound to one workspace's API key.
func NewClient(apiKey string) *Client {
	return &Client{apiKey: apiKey, http: &http.Client{Timeout: requestTimeout}}
}

// Request performs a call and returns the decoded JSON object. A non-2xx
// response yields an *APIError carrying the parsed Apollo message.
func (c *Client) Request(flow *core.Flow, method, path string, query url.Values, body interface{}) (map[string]interface{}, error) {
	u := BaseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}

	var reqBody io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(reqContext(flow), method, u, reqBody)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Api-Key", c.apiKey)
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &APIError{Status: resp.StatusCode, Body: respBody}
	}

	var decoded map[string]interface{}
	if len(respBody) > 0 {
		if err := json.Unmarshal(respBody, &decoded); err != nil {
			return nil, fmt.Errorf("unable to parse Apollo response: %w", err)
		}
	}
	return decoded, nil
}

// Post/Patch/Get are thin wrappers over Request for readability in actions.
func (c *Client) Post(flow *core.Flow, path string, body interface{}) (map[string]interface{}, error) {
	return c.Request(flow, http.MethodPost, path, nil, body)
}

func (c *Client) Patch(flow *core.Flow, path string, body interface{}) (map[string]interface{}, error) {
	return c.Request(flow, http.MethodPatch, path, nil, body)
}

func (c *Client) Get(flow *core.Flow, path string, query url.Values) (map[string]interface{}, error) {
	return c.Request(flow, http.MethodGet, path, query, nil)
}

// APIError is a non-2xx Apollo response, parsed for a human-readable message.
type APIError struct {
	Status int
	Body   []byte
}

func (e *APIError) Error() string { return e.Message() }

// Message pulls the best available text from Apollo's varied error shapes
// ({"error":"…"}, {"error_message":"…"}, {"errors":["…"]}), falling back to the
// raw body / status code.
func (e *APIError) Message() string {
	var parsed map[string]interface{}
	if json.Unmarshal(e.Body, &parsed) == nil {
		for _, k := range []string{"error", "error_message", "message"} {
			if s, ok := parsed[k].(string); ok && s != "" {
				return s
			}
		}
		if arr, ok := parsed["errors"].([]interface{}); ok && len(arr) > 0 {
			parts := make([]string, 0, len(arr))
			for _, a := range arr {
				if s, ok := a.(string); ok && s != "" {
					parts = append(parts, s)
				}
			}
			if len(parts) > 0 {
				return strings.Join(parts, "; ")
			}
		}
	}
	if s := strings.TrimSpace(string(e.Body)); s != "" {
		return fmt.Sprintf("Apollo API error (HTTP %d): %s", e.Status, s)
	}
	return fmt.Sprintf("Apollo API error (HTTP %d)", e.Status)
}

// AuthInputs is the shared credential input every Apollo action embeds first.
var AuthInputs = []core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
}

// NOTE: outputs are declared as inline literals inside each action (NOT a shared
// var), because the manifest generator only resolves inline composite literals —
// a cross-package Outputs reference yields empty manifest outputs and the editor
// draws no source handle. See the QBO/Xero note in CLAUDE.md.

// --- input helpers ---

func GetAPIKey(inputs []*core.Connection) (string, error) {
	key, err := RequiredString("api_key", inputs)
	if err != nil {
		return "", fmt.Errorf("an Apollo API key is required")
	}
	if strings.HasPrefix(key, "${") {
		return "", fmt.Errorf("the Apollo API key did not resolve — set an environment secret and reference it as ${secrets.X}")
	}
	return key, nil
}

func RequiredString(name string, inputs []*core.Connection) (string, error) {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil || *c.String() == "" {
		return "", fmt.Errorf("%s is required", name)
	}
	return *c.String(), nil
}

func OptionalString(name string, inputs []*core.Connection) string {
	c := core.FindConnection(name, inputs)
	if c == nil || c.String() == nil {
		return ""
	}
	return *c.String()
}

func OptionalInt(name string, inputs []*core.Connection) *int64 {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	return c.Number()
}

func OptionalBool(name string, inputs []*core.Connection) *bool {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return nil
	}
	return c.Boolean()
}

// BoolValue reads a boolean input as a plain bool, treating absent/unset as
// false — matching Apollo's own defaults for the reveal_* flags.
func BoolValue(name string, inputs []*core.Connection) bool {
	return BoolValueDefault(name, inputs, false)
}

// BoolValueDefault reads a boolean input, falling back to def when the input is
// absent or holds no interpretable value.
//
// The three states matter here. A boolean input is nil until someone touches it
// and a real bool afterwards, so "never configured" stays distinguishable from
// "deliberately switched off" — which is what lets a flag default to ON while
// still honouring an author who turns it off.
func BoolValueDefault(name string, inputs []*core.Connection, def bool) bool {
	if v := OptionalBool(name, inputs); v != nil {
		return *v
	}
	return def
}

// StringList splits a comma-separated input (e.g. contact_ids) into a trimmed
// slice, dropping blanks. Returns nil when the input is absent/blank.
func StringList(name string, inputs []*core.Connection) []string {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// LocationList splits a location input into Apollo location values.
//
// It splits on SEMICOLONS (and newlines), never on commas, because a comma is
// part of an Apollo location value rather than a separator between them:
// Apollo expects "Chester, United Kingdom" or "California, US" as a SINGLE
// string.
//
// Splitting locations on commas — as StringList does, and as this field used to
// — is not a cosmetic bug. Apollo ORs the values within an array filter, so
// "Chester, United Kingdom" became "Chester OR United Kingdom": the country
// clause swallows the city and the search quietly widens to the entire UK.
// The giveaway is that two different queries collapse to the same thing —
// "Chester, United Kingdom" and "Cheshire, United Kingdom" both reduce to
// "<somewhere> OR United Kingdom" and return an identical page of large UK
// employers. That reads as "the location filter is broken and returns national
// results", which is precisely how it was reported, when in fact the filter was
// working on the values we sent it.
//
// This mirrors RangeList, which already splits on ';' for the same underlying
// reason (a comma is meaningful *inside* each headcount range).
func LocationList(name string, inputs []*core.Connection) []string {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == '\n' || r == '\r' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// AddQueryLocationList adds a location input as repeated bracketed params,
// preserving the comma inside each location value.
func AddQueryLocationList(q url.Values, key, name string, inputs []*core.Connection) {
	for _, v := range LocationList(name, inputs) {
		q.Add(key+"[]", v)
	}
}

// --- body setters (assign into the request body only when present) ---

func SetString(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalString(name, inputs); v != "" {
		body[field] = v
	}
}

func SetInt(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalInt(name, inputs); v != nil {
		body[field] = *v
	}
}

func SetBool(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := OptionalBool(name, inputs); v != nil {
		body[field] = *v
	}
}

// SetList assigns a comma-separated input as a JSON array when non-empty.
func SetList(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := StringList(name, inputs); len(v) > 0 {
		body[field] = v
	}
}

// RangeList splits an input into range strings on SEMICOLONS and newlines only,
// preserving the comma INSIDE each range. Apollo's headcount filter expects an
// array of "min,max" strings (e.g. ["50,5000"]), so a plain comma-split would
// wrongly break "50,5000" into two elements. Returns nil when absent/blank.
func RangeList(name string, inputs []*core.Connection) []string {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil
	}
	parts := strings.FieldsFunc(raw, func(r rune) bool { return r == ';' || r == '\n' || r == '\r' })
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// SetRangeList assigns a semicolon-separated input as a JSON array of range
// strings (each "min,max") when non-empty.
func SetRangeList(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	if v := RangeList(name, inputs); len(v) > 0 {
		body[field] = v
	}
}

// SetNumberValue parses a decimal input (e.g. deal amount) as a JSON number.
func SetNumberValue(body map[string]interface{}, field, name string, inputs []*core.Connection) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return
	}
	if f, err := strconv.ParseFloat(strings.Map(stripMoneyRune, raw), 64); err == nil {
		body[field] = f
	}
}

func stripMoneyRune(r rune) rune {
	switch r {
	case '£', '$', '€', '¥', ',', ' ', '\t':
		return -1
	}
	return r
}

// ParseJSONObject reads a text input as a JSON object (advanced `fields`
// override). Returns nil when absent so nothing is merged.
func ParseJSONObject(name string, inputs []*core.Connection) (map[string]interface{}, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("invalid JSON in %s: %w", name, err)
	}
	return m, nil
}

// ParseJSONArray reads a text input as a JSON array (e.g. bulk match details).
// Returns nil when absent.
func ParseJSONArray(name string, inputs []*core.Connection) ([]interface{}, error) {
	raw := strings.TrimSpace(OptionalString(name, inputs))
	if raw == "" {
		return nil, nil
	}
	var a []interface{}
	if err := json.Unmarshal([]byte(raw), &a); err != nil {
		return nil, fmt.Errorf("invalid JSON array in %s: %w", name, err)
	}
	return a, nil
}

// MergeFields overlays extra JSON fields onto a body map (advanced overrides).
func MergeFields(body, extra map[string]interface{}) {
	for k, v := range extra {
		body[k] = v
	}
}

// --- response extraction ---

// Obj pulls a named object out of an Apollo response ({"person":{…}}).
func Obj(resp map[string]interface{}, key string) map[string]interface{} {
	obj, _ := resp[key].(map[string]interface{})
	return obj
}

// Arr pulls a named array of objects out of a response ({"people":[…]}).
func Arr(resp map[string]interface{}, key string) []map[string]interface{} {
	arr, ok := resp[key].([]interface{})
	if !ok {
		return nil
	}
	out := make([]map[string]interface{}, 0, len(arr))
	for _, e := range arr {
		if m, ok := e.(map[string]interface{}); ok {
			out = append(out, m)
		}
	}
	return out
}

// IDOf reads the "id" field of an Apollo object.
func IDOf(obj map[string]interface{}) string {
	if obj == nil {
		return ""
	}
	id, _ := obj["id"].(string)
	return id
}

// --- result shapers ---

// toolResultWithData renders the AI-facing tool_result as the summary followed
// by the JSON payload. This is load-bearing: when an action is invoked as an AI
// tool the agent receives ONLY tool_result (the `result`/`results` outputs are
// for downstream flow nodes), so a summary-only tool_result — e.g. "Found 10
// people" — starves the agent of the actual records. The flow engine truncates
// oversized tool results by token budget, so including the full payload is safe.
func toolResultWithData(summary string, data interface{}) string {
	b, err := json.Marshal(data)
	if err != nil || len(b) == 0 || string(b) == "null" || string(b) == "{}" || string(b) == "[]" {
		return summary
	}
	return summary + ":\n" + string(b)
}

// ObjectResult wraps a single Apollo object result for downstream nodes. The
// object is embedded in tool_result so an AI caller gets the fields, not just
// the summary.
func ObjectResult(id string, obj map[string]interface{}, summary string) map[string]interface{} {
	if obj == nil {
		obj = map[string]interface{}{}
	}
	if id == "" {
		id = IDOf(obj)
	}
	return map[string]interface{}{
		"tool_result": toolResultWithData(summary, obj),
		"id":          id,
		"result":      obj,
		"success":     true,
		"error":       "",
	}
}

// ListResult wraps a collection of Apollo objects. The records are embedded in
// tool_result so an AI caller receives the actual data (names, emails, …), not
// just a count.
func ListResult(items []map[string]interface{}, summary string) map[string]interface{} {
	if items == nil {
		items = []map[string]interface{}{}
	}
	return map[string]interface{}{
		"tool_result": toolResultWithData(summary, items),
		"results":     items,
		"count":       len(items),
		"success":     true,
		"error":       "",
	}
}

// PeopleProvenance renders one explicit line per person separating WHERE THE
// PERSON IS from WHERE THEIR EMPLOYER IS, plus the email and its Apollo status.
//
// The raw Apollo record contains both, but the person's own city sits at the
// top level while the employer's sits nested under "organization" — so a reader
// scanning the JSON easily reads a company HQ as though it confirmed the
// individual. That conflation is exactly what a "verified local" standard must
// not make: a company headquartered in Wrexham tells you nothing about where any
// given employee actually sits.
//
// Missing values are stated as "not provided" rather than omitted, so an absent
// field is visibly absent instead of quietly reading as agreement with whatever
// sits next to it.
func PeopleProvenance(records []map[string]interface{}) string {
	if len(records) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("PER-PERSON PROVENANCE (person location and company location are SEPARATE claims — do not treat a company HQ as confirming the individual):\n")
	for i, m := range records {
		name := strings.TrimSpace(strings.TrimSpace(fieldStr(m, "first_name")) + " " + strings.TrimSpace(fieldStr(m, "last_name")))
		if ob := strings.TrimSpace(fieldStr(m, "last_name_obfuscated")); ob != "" {
			name = strings.TrimSpace(fieldStr(m, "first_name")) + " " + ob + " (SURNAME WITHHELD BY PLAN)"
		}
		if strings.TrimSpace(name) == "" {
			name = "(name not provided)"
		}

		org := Obj(m, "organization")
		fmt.Fprintf(&b, "%d. %s — %s @ %s | person location: %s | company location: %s | email: %s (status: %s)\n",
			i+1,
			name,
			orNotProvided(fieldStr(m, "title")),
			orNotProvided(fieldStr(org, "name")),
			orNotProvided(joinLocation(fieldStr(m, "city"), fieldStr(m, "state"), fieldStr(m, "country"))),
			orNotProvided(joinLocation(fieldStr(org, "city"), fieldStr(org, "state"), fieldStr(org, "country"))),
			orNotProvided(fieldStr(m, "email")),
			orNotProvided(fieldStr(m, "email_status")),
		)
	}
	return b.String()
}

// fieldStr reads a string field, tolerating a non-string or absent value.
func fieldStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	s, _ := m[key].(string)
	return strings.TrimSpace(s)
}

func joinLocation(parts ...string) string {
	kept := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			kept = append(kept, t)
		}
	}
	return strings.Join(kept, ", ")
}

// minResultsForLocationCheck is the smallest result count worth judging. Below
// this a zero-match run is just as likely to be a genuinely sparse area as an
// ignored filter, and a warning would be noise.
const minResultsForLocationCheck = 3

// LocationIgnoredWarning reports when a location filter appears to have had no
// effect, because NONE of the returned records mention the requested location.
//
// Apollo silently ignores location values outside its own taxonomy. It indexes
// city, state/region and country; many UK counties are not in it. Ask for
// "Cheshire, United Kingdom" and Apollo does not error or return nothing — it
// drops the constraint and answers an unfiltered, relevance-ranked search. The
// result is a plausible-looking page of large national employers (The Economist,
// the FT, Robert Walters) that reads as a successful county search and is
// nothing of the kind. That is the same silent-wrong-answer shape as the
// comma-splitting bug, arriving by a different route.
//
// The check is deliberately conservative. Apollo's city filters legitimately
// include surrounding towns — a "Chester" search returning Ellesmere Port and
// Capenhurst is correct behaviour, not a miss — so this only speaks up when
// literally nothing matches, and it reports the count as a fact and the cause as
// a possibility rather than asserting breakage. A record's own location may also
// simply be absent, so a warning is a prompt to verify, never a verdict.
func LocationIgnoredWarning(requested []string, recordLocations []string) string {
	needles := locationNeedles(requested)
	if len(needles) == 0 {
		return ""
	}

	known, matched := countLocationMatches(needles, recordLocations)
	// Only judge records that actually STATE a location. Counting a withheld
	// location as a non-match is how this reported a working filter as broken:
	// on a plan that masks personal data every person's city is null, so every
	// record "fails" to match and the warning fires on results that may be
	// perfectly scoped. Absence of evidence is not evidence of absence.
	if known < minResultsForLocationCheck || matched > 0 {
		return ""
	}

	return fmt.Sprintf(
		"WARNING - LOCATION FILTER MAY HAVE BEEN IGNORED: none of the %d result(s) that state a location report the requested one (%s). "+
			"Apollo silently drops location values it does not recognise and answers an UNFILTERED search instead, which looks like a normal result. "+
			"Apollo indexes city, state/region and country — UK counties (e.g. \"Cheshire\") are often absent, so prefer a city such as \"Chester, United Kingdom\" "+
			"and use ; to add neighbouring towns. Verify the geography of these results before treating them as local.",
		known, strings.Join(requested, "; "))
}

// PersonLocationNote describes what can and cannot be concluded about geography
// when a person's own location is withheld by the plan.
//
// This exists because the honest answer in that case is "unknown", and the two
// wrong answers are both costly. Saying nothing invites a reader to assume the
// filter worked and treat national title-matches as locals. Warning that the
// filter was ignored invites the opposite — writing off results that were
// correctly scoped — which is what happened: a Chester people search returned
// Practice Plan, Oxbury Bank and i6 Group, all genuinely Chester employers, and
// was reported as "location filter still ignored" purely because every person's
// city had been masked.
//
// So when person locations are unavailable it falls back to the EMPLOYER's
// location, which the plan does not mask, and labels it as exactly that: a
// company-level signal, not confirmation of where any individual sits.
func PersonLocationNote(requested []string, personLocations, orgLocations []string) string {
	needles := locationNeedles(requested)
	if len(needles) == 0 {
		return ""
	}

	knownPeople, _ := countLocationMatches(needles, personLocations)
	if knownPeople >= minResultsForLocationCheck {
		// Person locations are available; LocationIgnoredWarning covers this.
		return ""
	}

	withheld := len(personLocations)
	if withheld == 0 {
		return ""
	}

	knownOrgs, matchedOrgs := countLocationMatches(needles, orgLocations)
	if knownOrgs == 0 {
		return fmt.Sprintf(
			"NOTE - GEOGRAPHY UNVERIFIABLE: none of the %d result(s) report a location for the person OR their employer, so whether the location filter (%s) took effect cannot be determined from this response. "+
				"On plans that mask personal data, a person's city is withheld in search results. Enrich individual records to obtain a verified location.",
			withheld, strings.Join(requested, "; "))
	}

	if matchedOrgs == 0 {
		return fmt.Sprintf(
			"WARNING - LIKELY NOT LOCAL: the people's own locations are withheld by this key's plan, and none of the %d employer(s) that state a location are in the requested area (%s). "+
				"That is the pattern of an ignored location filter returning national title-matches. Treat these as unverified.",
			knownOrgs, strings.Join(requested, "; "))
	}

	return fmt.Sprintf(
		"NOTE - PERSON LOCATION WITHHELD, EMPLOYER LOCATION MATCHES: the people's own cities are masked by this key's plan, so local residency is UNCONFIRMED — but %d of the %d employer(s) that state a location are in the requested area (%s), which indicates the filter did take effect. "+
			"A company being local does not make a given employee local; enrich individual records to confirm a person's city before treating them as local.",
		matchedOrgs, knownOrgs, strings.Join(requested, "; "))
}

// locationNeedles lowercases each requested location and also its leading
// segment, since a record reports city and country in separate fields while the
// filter is written "City, Country".
func locationNeedles(requested []string) []string {
	needles := make([]string, 0, len(requested)*2)
	for _, r := range requested {
		full := strings.ToLower(strings.TrimSpace(r))
		if full == "" {
			continue
		}
		needles = append(needles, full)
		if head, _, found := strings.Cut(full, ","); found {
			if h := strings.TrimSpace(head); h != "" {
				needles = append(needles, h)
			}
		}
	}
	return needles
}

// countLocationMatches returns how many records STATE a location, and how many
// of those match. The first count is the denominator that matters: a record with
// no location is unknown, not a miss.
func countLocationMatches(needles []string, locations []string) (known, matched int) {
	for _, loc := range locations {
		l := strings.ToLower(strings.TrimSpace(loc))
		if l == "" {
			continue
		}
		known++
		for _, n := range needles {
			if strings.Contains(l, n) {
				matched++
				break
			}
		}
	}
	return known, matched
}

// PeopleOrgLocations renders the EMPLOYER's location for each person record —
// read from the nested "organization" object, not the person's own fields.
// Used only as a fallback signal when the person's own location is withheld.
func PeopleOrgLocations(records []map[string]interface{}) []string {
	out := make([]string, 0, len(records))
	for _, m := range records {
		org := Obj(m, "organization")
		out = append(out, joinLocation(fieldStr(org, "city"), fieldStr(org, "state"), fieldStr(org, "country")))
	}
	return out
}

// OrgLocations renders each organisation's own location for the filter check.
func OrgLocations(records []map[string]interface{}) []string {
	out := make([]string, 0, len(records))
	for _, m := range records {
		out = append(out, joinLocation(fieldStr(m, "city"), fieldStr(m, "state"), fieldStr(m, "country")))
	}
	return out
}

// PersonLocations renders each person's own location — NOT their employer's —
// for the filter check, since person_locations filters on the individual.
func PersonLocations(records []map[string]interface{}) []string {
	out := make([]string, 0, len(records))
	for _, m := range records {
		out = append(out, joinLocation(fieldStr(m, "city"), fieldStr(m, "state"), fieldStr(m, "country")))
	}
	return out
}

func orNotProvided(s string) string {
	if strings.TrimSpace(s) == "" {
		return "not provided"
	}
	return s
}

// ErrorResult is a graceful failure — success=false, not a node error — so an
// invalid parameter or rate limit can be handled within the flow.
func ErrorResult(msg string) map[string]interface{} {
	return map[string]interface{}{
		"tool_result": msg,
		"success":     false,
		"error":       msg,
	}
}

// MapError converts any request error into a graceful ErrorResult. A 429 is
// surfaced verbatim so the flow author sees Apollo's rate-limit message.
func MapError(err error) map[string]interface{} {
	if ae, ok := err.(*APIError); ok {
		return ErrorResult(ae.Message())
	}
	return ErrorResult(err.Error())
}

// ── Query-parameter builders ────────────────────────────────────────────────
//
// CRITICAL: Apollo's search endpoints (people, companies, contacts, accounts,
// sequences, emailer messages) read their filters from the URL QUERY STRING,
// not the JSON request body — array filters use bracket notation
// (key[]=a&key[]=b). Sending filters in the body makes Apollo silently ignore
// them and return a generic, unscoped list. Every search action must build its
// filters with these helpers and POST via PostQuery.

// PostQuery POSTs to path with the filters in the query string and an empty JSON
// body (which preserves the application/json Content-Type Apollo expects).
func (c *Client) PostQuery(flow *core.Flow, path string, query url.Values) (map[string]interface{}, error) {
	return c.Request(flow, http.MethodPost, path, query, map[string]interface{}{})
}

// AddQueryString sets a scalar query param from a string input, when non-empty.
func AddQueryString(q url.Values, key, name string, inputs []*core.Connection) {
	if v := strings.TrimSpace(OptionalString(name, inputs)); v != "" {
		q.Set(key, v)
	}
}

// AddQueryList adds an array input as repeated bracketed params: key[]=a&key[]=b.
func AddQueryList(q url.Values, key, name string, inputs []*core.Connection) {
	for _, v := range StringList(name, inputs) {
		if t := strings.TrimSpace(v); t != "" {
			q.Add(key+"[]", t)
		}
	}
}

// AddQueryRangeList adds a range input (e.g. "50,5000") as bracketed params,
// preserving the comma inside each range.
func AddQueryRangeList(q url.Values, key, name string, inputs []*core.Connection) {
	for _, v := range RangeList(name, inputs) {
		if t := strings.TrimSpace(v); t != "" {
			q.Add(key+"[]", t)
		}
	}
}

// AddQueryInt sets an integer query param when the input is present.
func AddQueryInt(q url.Values, key, name string, inputs []*core.Connection) {
	if p := OptionalInt(name, inputs); p != nil {
		q.Set(key, strconv.FormatInt(*p, 10))
	}
}

// AddQueryBool sets a boolean query param when the input is present.
func AddQueryBool(q url.Values, key, name string, inputs []*core.Connection) {
	if b := OptionalBool(name, inputs); b != nil {
		q.Set(key, strconv.FormatBool(*b))
	}
}

// AddQueryFromMap flattens an arbitrary JSON object (a `fields` override) into
// query params: scalars as key=value, arrays as key[]=item.
func AddQueryFromMap(q url.Values, m map[string]interface{}) {
	for k, v := range m {
		switch vv := v.(type) {
		case nil:
		case string:
			q.Set(k, vv)
		case bool:
			q.Set(k, strconv.FormatBool(vv))
		case float64:
			q.Set(k, strconv.FormatFloat(vv, 'f', -1, 64))
		case []interface{}:
			for _, item := range vv {
				q.Add(k+"[]", fmt.Sprint(item))
			}
		default:
			q.Set(k, fmt.Sprint(vv))
		}
	}
}

// ── Withheld / obfuscated personal data ─────────────────────────────────────
//
// Apollo returns people records whose personal data is WITHHELD: the surname
// comes back only as `last_name_obfuscated` ("Mc***y") and email/city/phone are
// replaced by `has_email` / `has_city` / `has_direct_phone` boolean flags with
// no actual value.
//
// The usual cause is NOT a billing problem. People Search is free and NEVER
// returns personal emails, by design — `has_email: true` is Apollo correctly
// saying "an email exists for this person, enrich to retrieve it". Emails come
// only from the enrichment endpoints, and only when `reveal_personal_emails` is
// explicitly set: it defaults to FALSE, so an enrichment call that omits it
// returns a null email while consuming no credit. Apollo charges 1 credit for a
// revealed email and 8 for a mobile, and only when data is actually found.
//
// Getting this attribution wrong is expensive in a way an empty result is not.
// Reading withheld data as "our plan is too limited" leads to abandoning the
// integration for a slower manual route, when the fix was a flag. So the warning
// leads with the flag and offers plan/credits only as the fallback explanation.
// See https://docs.apollo.io/reference/people-enrichment.

// IsGatedRecord reports whether an Apollo person record has plan-gated personal
// data (an obfuscated surname, or a has_* flag set while the real value is
// absent/empty).
func IsGatedRecord(m map[string]interface{}) bool {
	if m == nil {
		return false
	}
	if s, ok := m["last_name_obfuscated"].(string); ok && strings.TrimSpace(s) != "" {
		return true
	}
	if truthyFlag(m["has_email"]) && emptyValue(m["email"]) {
		return true
	}
	if truthyFlag(m["has_city"]) && emptyValue(m["city"]) {
		return true
	}
	return false
}

// GatePrefix returns summary with a withheld-data notice prepended when any of
// the records have personal data withheld; otherwise it returns summary
// unchanged. The notice goes into tool_result so an AI/agent caller cannot miss
// it — and, critically, states the LIKELY CAUSE correctly so the reader fixes a
// flag rather than concluding the integration is unusable.
func GatePrefix(summary string, records []map[string]interface{}) string {
	return gatePrefix(summary, records, enrichGateAdvice)
}

// GatePrefixSearch is the SEARCH variant of the same notice.
//
// The two endpoints withhold data for genuinely different reasons, and one
// message cannot serve both without misleading on one of them:
//
//   - Search: an email is NEVER returned — that is by design, not a limitation.
//     But a masked surname (last_name_obfuscated) or a null city IS a plan
//     limitation: Apollo's free and Basic tiers mask personal data in search
//     results. No parameter changes that.
//   - Enrichment: a null email is normally the reveal flag, which defaults off
//     at Apollo (our actions default it on).
//
// Sending the enrichment advice on search results told a reader that masked
// surnames were "usually not a plan problem" and to set a reveal flag that
// search does not have — advice that is both wrong and unactionable there.
func GatePrefixSearch(summary string, records []map[string]interface{}) string {
	return gatePrefix(summary, records, searchGateAdvice)
}

const enrichGateAdvice = "A null email here is USUALLY the reveal flag rather than the plan: Apollo does not return personal emails unless asked, and its own default is not to. " +
	"These actions set Reveal Personal Emails ON by default, so if it is still null either the flag was switched off on this node, the record genuinely has no email, or the key's credits are exhausted. " +
	"Apollo charges 1 credit per revealed email (8 for a mobile), only when data is found."

const searchGateAdvice = "Search NEVER returns a personal email — that is by design, not a limitation, so has_email:true simply means an email exists and can be retrieved by enriching. " +
	"A masked surname or a null city, however, IS a plan limitation: Apollo's free and Basic tiers mask personal data in SEARCH results, and no parameter changes that. " +
	"Call People: Enrich (people/match) on the records you want, which returns the full name, location and email (1 credit per email, charged only when found)."

func gatePrefix(summary string, records []map[string]interface{}, advice string) string {
	gated := 0
	for _, m := range records {
		if IsGatedRecord(m) {
			gated++
		}
	}
	if gated == 0 {
		return summary
	}
	warn := fmt.Sprintf(
		"NOTE - PERSONAL DATA WITHHELD: %d of %d record(s) have a masked surname (last_name_obfuscated) or a has_email/has_city flag with no value. %s "+
			"Until resolved, do not present these as confirmed contacts.",
		gated, len(records), advice)
	if strings.TrimSpace(summary) == "" {
		return warn
	}
	return warn + "\n\n" + summary
}

// RevealHint explains a missing email, distinguishing the two very different
// reasons for one.
//
// An enrichment that returns no email is ambiguous: it can mean "this person has
// no email on file" or "you asked us not to fetch it". Only the second is
// actionable, and conflating them is what turns a one-switch fix into a
// conclusion that the data is unavailable. Apollo's own default is not to
// reveal, so this action defaults reveal ON — which means the second case now
// only arises when an author has deliberately switched it off.
func RevealHint(person map[string]interface{}, revealRequested bool) string {
	if person == nil || !emptyValue(person["email"]) {
		return ""
	}
	if revealRequested {
		return "No email returned even though Reveal Personal Emails was on — Apollo has no personal email on file for this person, or the key's credits are exhausted. No credit is charged when nothing is found."
	}
	return "No email returned because Reveal Personal Emails is switched OFF on this node. Apollo does not return personal emails unless asked. Switch it on to retrieve one — 1 credit, charged only if an email is found."
}

// truthyFlag reads Apollo's has_* flags, which arrive as either a bool or a
// string ("true"/"Yes").
func truthyFlag(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "yes"
	}
	return false
}

func emptyValue(v interface{}) bool {
	if v == nil {
		return true
	}
	if s, ok := v.(string); ok {
		return strings.TrimSpace(s) == ""
	}
	return false
}
