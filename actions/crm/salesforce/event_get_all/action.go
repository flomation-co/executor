package crm_salesforce_event_get_all

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Events"
	Description  = "See what is in the diary — every Salesforce event in a date range, on one person's calendar, or booked against a particular customer or deal."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields To Return", Placeholder: "Id, Subject, StartDateTime, Location — leave blank for the usual ones"},
	{Name: "starts_after", Type: core.ConnectionTypeDateTime, Label: "Starts On or After", Placeholder: "Beginning of the period you want to see"},
	{Name: "starts_before", Type: core.ConnectionTypeDateTime, Label: "Starts On or Before", Placeholder: "End of the period you want to see"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Whose Calendar", Placeholder: "Salesforce user ID of the person the events belong to"},
	{Name: "who_id", Type: core.ConnectionTypeString, Label: "With Contact or Lead", Placeholder: "Record ID of the person being met"},
	{Name: "what_id", Type: core.ConnectionTypeString, Label: "Related Record", Placeholder: "Record ID of the account, opportunity or case"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "Any other field to filter on, e.g. Location"},
	{
		Name:        "filter_operator",
		Type:        core.ConnectionTypeString,
		Label:       "Filter Comparison",
		Placeholder: "How to compare the field (defaults to Equals)",
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "="},
			{Name: "Does Not Equal", Value: "!="},
			{Name: "Less Than", Value: "<"},
			{Name: "Less Than Or Equal To", Value: "<="},
			{Name: "Greater Than", Value: ">"},
			{Name: "Greater Than Or Equal To", Value: ">="},
			{Name: "Contains (use % as the wildcard)", Value: "LIKE"},
			{Name: "Does Not Contain (use % as the wildcard)", Value: "NOT LIKE"},
			{Name: "Is One Of (comma-separated)", Value: "IN"},
			{Name: "Is Not One Of (comma-separated)", Value: "NOT IN"},
		},
	},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "Head office — for Contains use % as the wildcard (%head office%); or a date shortcut like THIS_WEEK"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"IsAllDayEvent","operator":"=","value":"false"}]`},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all", Placeholder: "On, matching any one of the filters is enough. The Starts On or After / Starts On or Before dates always apply"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "StartDateTime ASC"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000) — ignored when Return All is on"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Cancelled and Archived Events"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Events"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Records Returned"},
	{Name: "next_url", Type: core.ConnectionTypeString, Label: "Next Page URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// Every filter — the plain-English boxes and the JSON escape hatch alike —
	// becomes a Condition and goes through the shared query builder, which is
	// the one place identifiers are whitelisted and values are escaped. Nothing
	// here builds SOQL by hand.
	filters, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	if field := salesforce.OptionalString("filter_field", inputs); field != "" {
		filters = append(filters, salesforce.Condition{
			Field:    field,
			Operator: salesforce.OptionalString("filter_operator", inputs),
			Value:    salesforce.OptionalString("filter_value", inputs),
		})
	}
	filters = appendEqualsFilter(filters, "OwnerId", salesforce.OptionalString("owner_id", inputs))
	filters = appendEqualsFilter(filters, "WhoId", salesforce.OptionalString("who_id", inputs))
	filters = appendEqualsFilter(filters, "WhatId", salesforce.OptionalString("what_id", inputs))

	// The two ends of the range are not symmetrical when the operator gives a
	// bare date — see soqlDateTime. The start of the period wants midnight; the
	// end of it wants the last second of that day.
	//
	// They are also SCOPE rather than filters: the pair reads as one control
	// bounding the period on view, so "Match ANY filter instead of all" must not
	// be allowed to OR them. `StartDateTime >= X OR StartDateTime <= Y` is true
	// of every event in the diary, which turned a week's calendar lookup into
	// the whole org's history while reporting success.
	var scope []salesforce.Condition
	after, err := soqlDateTime(salesforce.OptionalString("starts_after", inputs), false)
	if err != nil {
		return nil, err
	}
	if after != "" {
		scope = append(scope, salesforce.Condition{Field: "StartDateTime", Operator: ">=", Value: after})
	}
	before, err := soqlDateTime(salesforce.OptionalString("starts_before", inputs), true)
	if err != nil {
		return nil, err
	}
	if before != "" {
		scope = append(scope, salesforce.Condition{Field: "StartDateTime", Operator: "<=", Value: before})
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	// BuildQueryTyped, not BuildQuery: the filter boxes carry operator-supplied
	// values, and whether each one is quoted depends on the FIELD's Salesforce
	// type — IsAllDayEvent = false and DurationInMinutes > 30 are both bare, and
	// both are INVALID_FIELD if quoted. One cached describe settles it, and a
	// describe the connected user is not allowed to run simply falls back.
	soql, err := salesforce.BuildScopedQueryTyped(
		instanceURL,
		token,
		"Event",
		salesforce.OptionalString("fields", inputs),
		scope,
		filters,
		salesforce.OptionalBool("match_any_filter", inputs),
		salesforce.OptionalString("order_by", inputs),
		salesforce.ClampLimit(limit, limitSet),
		!returnAll,
	)
	if err != nil {
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, salesforce.OptionalBool("include_deleted", inputs))
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d event(s) across %d page(s); stopped at the %d-page safety cap — narrow the date range to see the rest", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d event(s) across %d page(s)", len(records), pages)
	default:
		// Salesforce reports totalSize as the PAGE size once a LIMIT is applied
		// and sets done:true with no cursor, so a capped list is otherwise
		// indistinguishable from a complete one.
		out["tool_result"] = fmt.Sprintf("Found %d event(s)%s", len(records), salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}

// appendEqualsFilter adds a simple "field = value" term when the operator filled
// the box in. Building these as Conditions rather than as query text keeps every
// operator-supplied value inside the shared escaping boundary.
func appendEqualsFilter(conditions []salesforce.Condition, field, value string) []salesforce.Condition {
	if strings.TrimSpace(value) == "" {
		return conditions
	}
	return append(conditions, salesforce.Condition{Field: field, Operator: "=", Value: value})
}

// dateOnlyLayout is the one entry in dateTimeLayouts carrying no time of day.
// Which layout matched has to survive parsing, because a bare date means two
// different things at the two ends of a range.
const dateOnlyLayout = "2006-01-02"

// dateTimeLayouts are the shapes a date picker, a spreadsheet or an upstream
// node can realistically hand us for a moment in time.
var dateTimeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02T15:04:05",
	"2006-01-02T15:04",
	"2006-01-02 15:04:05",
	"2006-01-02 15:04",
	dateOnlyLayout,
}

// soqlDateTime normalises a date-and-time input into the only form SOQL accepts
// for a DateTime comparison: a full ISO-8601 timestamp carrying a zone.
// "2026-07-25T09:00" quietly fails to match Salesforce's literal pattern, gets
// quoted as a string instead, and the query then errors — so anything short is
// completed here. A value with no zone is read as UTC, which is the assumption
// worth stating out loud since it shifts results for a UK org in summer.
//
// endOfDay changes what a BARE DATE means, and only a bare date. StartDateTime
// is a DateTime, so "on or before 24 July" typed as 2026-07-24 parses to
// midnight — the instant the 24th begins — and the upper bound then silently
// drops every appointment that day, which is precisely the last day of the
// range the operator asked to see. A "this week" diary lookup would lose its
// final day and report success. The lower bound genuinely wants midnight and is
// left alone; anything carrying a time of day is used exactly as given.
func soqlDateTime(raw string, endOfDay bool) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", nil
	}
	for _, layout := range dateTimeLayouts {
		t, err := time.Parse(layout, v)
		if err != nil {
			continue
		}
		if endOfDay && layout == dateOnlyLayout {
			// Parsed as UTC, so this is plain arithmetic — 23:59:59 the same day,
			// with no DST edge to fall down.
			t = t.Add(24*time.Hour - time.Second)
		}
		return t.UTC().Format("2006-01-02T15:04:05Z"), nil
	}
	return "", fmt.Errorf("%q is not a date and time we can read — use something like 2026-07-25T09:00:00Z", raw)
}
