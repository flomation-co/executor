package crm_salesforce_search_records

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Find Records"
	Description  = "Search Salesforce for anything — a name, an email address, a phone number, a reference — and get back the matching records across whichever objects you choose. No query writing required."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "search_term", Type: core.ConnectionTypeString, Label: "Search For", Placeholder: "acme, jane@example.com, 07700 900123 — at least 2 characters", Required: true},
	{Name: "objects", Type: core.ConnectionTypeString, Label: "Search In These Records", Placeholder: "Account, Contact, Lead — leave blank to search everything"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Details to Return", Placeholder: "Id, Name, Email — leave blank for the usual details"},
	{
		Name:        "search_scope",
		Type:        core.ConnectionTypeString,
		Label:       "Where to Look",
		Placeholder: "All searchable details",
		Options: []core.ConnectionOption{
			{Name: "All Searchable Details", Value: "ALL"},
			{Name: "Names Only", Value: "NAME"},
			{Name: "Email Addresses Only", Value: "EMAIL"},
			{Name: "Phone Numbers Only", Value: "PHONE"},
		},
	},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Maximum Results", Placeholder: "50 (max 2000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Matching Records"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Returned"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

// searchResponse is Salesforce's parameterizedSearch envelope. Unlike a SOQL
// query it carries no totalSize, done flag or cursor — the whole result set
// comes back in one response, bounded by overallLimit.
type searchResponse struct {
	SearchRecords []map[string]interface{} `json:"searchRecords"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	term, err := salesforce.RequiredString("search_term", inputs)
	if err != nil {
		return nil, err
	}
	// Salesforce rejects a one-character search outright. Catching it here turns
	// a raw INVALID_SEARCH into something the operator can act on, and costs no
	// API call.
	if len([]rune(term)) < 2 {
		return nil, fmt.Errorf("search for at least 2 characters — Salesforce cannot search on %q", term)
	}

	q := url.Values{}
	q.Set("q", EscapeSearchTerm(term))

	// The object list is optional: with no sobject parameter Salesforce searches
	// every object the connected user can see, which is the right default for
	// someone who just wants to find "that Acme thing".
	objects, err := validatedObjects(inputs)
	if err != nil {
		return nil, err
	}
	fields, err := validatedFields(inputs)
	if err != nil {
		return nil, err
	}

	if len(objects) > 0 {
		for _, obj := range objects {
			// One sobject parameter PER object, repeated — that is the form
			// Salesforce documents ("?q=Smith&sobject=Contact&Contact.fields=…
			// &sobject=Lead&Lead.fields=…"). A single comma-joined value is the
			// obvious-looking alternative and is not what the endpoint parses.
			q.Add("sobject", obj)
			// parameterizedSearch returns nothing but record IDs unless fields
			// are named, which would make the action useless on its own. When
			// the operator has not chosen any, fall back to the same sensible
			// per-object list the get-many actions use.
			list := fields
			if list == "" {
				// Resolve from describe rather than guessing: this action is
				// pointed at an arbitrary object by the operator, and the
				// static fallback's "Name" is a hard INVALID_FIELD on Task,
				// Event, Case, ContentDocument and every junction object.
				list = salesforce.DefaultFieldsFor(instanceURL, token, obj)
			}
			q.Set(obj+".fields", list)
		}
	} else if fields != "" {
		// Without an object list the fields apply to every object matched, so
		// they must exist on all of them — Salesforce errors otherwise. Only set
		// what the operator explicitly asked for; guessing here would break the
		// blank-object case for any org whose objects differ.
		q.Set("fields", fields)
	}

	if scope := salesforce.OptionalString("search_scope", inputs); scope != "" {
		q.Set("in", strings.ToUpper(scope))
	}

	// overallLimit is always sent. An unbounded search on a large org returns a
	// lot of rows the operator did not ask for, and this is a shared API
	// allowance the customer's other integrations are drawing on too.
	limit, set := salesforce.OptionalInt("limit", inputs)
	overallLimit := salesforce.ClampLimit(limit, set)
	q.Set("overallLimit", strconv.Itoa(overallLimit))

	resp, err := salesforce.ExecuteAPI(instanceURL, token, http.MethodGet, "/parameterizedSearch/?"+q.Encode(), nil)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}
	if err := salesforce.CheckResponse(resp); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	var payload searchResponse
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return salesforce.ErrorResult(fmt.Sprintf("failed to parse Salesforce search response: %s", err)), nil
	}

	records := payload.SearchRecords
	if records == nil {
		records = []map[string]interface{}{}
	}

	// There is no cursor on this endpoint — everything Salesforce is going to
	// return is in this one response — so next_url stays empty and total_size
	// is simply what came back.
	out := salesforce.ListResult(records, "", len(records), "")
	// idsOnly: with neither an object list nor a field list there is nothing to
	// hang a fields parameter off, so Salesforce answers with bare IDs. That is
	// its documented default rather than a fault, but it surprises an operator
	// who expected names and email addresses, so the summary says so.
	idsOnly := len(objects) == 0 && fields == ""
	out["tool_result"] = summarise(records, term, overallLimit, idsOnly)
	return out, nil
}

// summarise describes the result in the operator's terms, naming the objects
// that matched (the whole point of a cross-object search is that they did not
// have to know which one the record lived on) and flagging a truncated result.
func summarise(records []map[string]interface{}, term string, overallLimit int, idsOnly bool) string {
	if len(records) == 0 {
		return fmt.Sprintf("Nothing matched %q — note that Salesforce's search index can take a few seconds to pick up brand-new records", term)
	}
	base := fmt.Sprintf("Found %d record(s) matching %q", len(records), term)
	if types := matchedTypes(records); types != "" {
		base += " in " + types
	}
	if len(records) >= overallLimit {
		base += fmt.Sprintf(" — stopped at the %d-result limit, so there may be more", overallLimit)
	}
	if idsOnly {
		base += " — only record IDs were returned; fill in Search In These Records, or Details to Return, to get names and other details"
	}
	return base
}

// matchedTypes lists which objects the matches came from, in the order they
// were returned. Each record carries its type in the attributes envelope
// Salesforce stamps on every search result.
func matchedTypes(records []map[string]interface{}) string {
	seen := map[string]bool{}
	var types []string
	for _, r := range records {
		attrs, ok := r["attributes"].(map[string]interface{})
		if !ok {
			continue
		}
		t, _ := attrs["type"].(string)
		if t == "" || seen[t] {
			continue
		}
		seen[t] = true
		types = append(types, t)
	}
	return strings.Join(types, ", ")
}

// validatedObjects reads and whitelist-validates the object list. An invalid
// object name is a configuration mistake, so it fails hard rather than being
// passed to Salesforce to reject.
func validatedObjects(inputs []*core.Connection) ([]string, error) {
	raw := salesforce.SplitList(salesforce.OptionalString("objects", inputs))
	out := make([]string, 0, len(raw))
	for _, name := range raw {
		obj, err := salesforce.ValidateSOQLObjectName(name)
		if err != nil {
			return nil, err
		}
		out = append(out, obj)
	}
	return out, nil
}

// validatedFields reads and whitelist-validates the field list, returning it in
// the comma-separated form the search parameters expect.
func validatedFields(inputs []*core.Connection) (string, error) {
	raw := salesforce.SplitList(salesforce.OptionalString("fields", inputs))
	out := make([]string, 0, len(raw))
	for _, name := range raw {
		field, err := salesforce.ValidateSOQLFieldName(name)
		if err != nil {
			return "", err
		}
		out = append(out, field)
	}
	return strings.Join(out, ","), nil
}

// soslReserved are the characters Salesforce's search parser treats as syntax.
// An unescaped one in an ordinary search — "O'Brien", "Acme (UK) Ltd", "A&B" —
// is not a security problem so much as a correctness one: Salesforce answers
// with a parse error and the operator has no idea why searching for a customer's
// own name failed.
//
// The wildcards * and ? are deliberately NOT escaped: "acme*" is a genuinely
// useful thing to type and matching Salesforce's own search box behaviour is
// worth more here than treating them literally.
const soslReserved = `&|!{}[]()^~:\"'+-`

// EscapeSearchTerm backslash-escapes the SOSL syntax characters in an
// operator's search string.
//
// It walks the term once and emits as it goes, rather than running a sequence
// of replacements the way EscapeSOQLString has to. That is deliberate: a
// replacement chain has to escape the backslash first or its own output gets
// escaped a second time, and a single pass cannot fall into that trap at all.
func EscapeSearchTerm(term string) string {
	var b strings.Builder
	b.Grow(len(term) + 8)
	for _, r := range term {
		if strings.ContainsRune(soslReserved, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}
