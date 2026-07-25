// Package crm_salesforce_lead_get_all lists Leads.
//
// Salesforce has no filterable REST list endpoint for records — listing means
// running a SOQL query — so this action builds one from operator-friendly parts
// rather than asking a receptionist to write SOQL. Every identifier is
// whitelist-validated and every value escaped by the shared builder, which is
// the injection boundary for the whole node.
//
// The filter comes in two layers on purpose. One field, one comparison and one
// value covers the overwhelming majority of real flows ("leads created this
// week", "leads from the web") with no JSON in sight; More Filters is there for
// the minority that need a second or third condition. Both layers feed the same
// validated builder.
//
// Two things this does that n8n's equivalent cannot: combine conditions with OR
// rather than AND only, and sort the results.
package crm_salesforce_lead_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Leads"
	Description  = "List leads, optionally filtered and sorted. Turn on Return All to fetch every match, page by page, instead of just the first batch."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	// Blank gives the standard shortlist (name, company, address, email,
	// status). A Salesforce lead has well over a hundred fields and returning
	// them all makes the run history unreadable, so ask for what you need.
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "FirstName, LastName, Email, Status — blank for the usual few"},

	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "Status, LeadSource, CreatedDate…"},
	{
		Name:        "filter_operator",
		Type:        core.ConnectionTypeString,
		Label:       "Filter Comparison",
		Placeholder: "Equals",
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "="},
			{Name: "Does not equal", Value: "!="},
			{Name: "Less than", Value: "<"},
			{Name: "Less than or equal to", Value: "<="},
			{Name: "Greater than", Value: ">"},
			{Name: "Greater than or equal to", Value: ">="},
			{Name: "Matches (use % as a wildcard)", Value: "LIKE"},
			{Name: "Does not match (use % as a wildcard)", Value: "NOT LIKE"},
			{Name: "Is one of (comma-separated)", Value: "IN"},
			{Name: "Is not one of (comma-separated)", Value: "NOT IN"},
		},
	},
	// Salesforce's own date words work here and are far friendlier than
	// calculating a date upstream: TODAY, THIS_WEEK, LAST_N_DAYS:7.
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "Open - Not Contacted — or TODAY, LAST_N_DAYS:7 for dates"},

	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"AnnualRevenue","operator":">","value":"250000"}]`},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},

	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "CreatedDate DESC, LastName ASC"},

	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},

	// /queryAll rather than /query. Recycle Bin records and archived activities
	// are invisible to a normal query, which is why leads look as though they
	// have "vanished" when somebody has deleted them.
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted and Archived"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Leads"},
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

	// A malformed filter, an unknown comparison or a field name that is not a
	// valid identifier are all configuration mistakes and fail hard — the query
	// is never sent, so there is nothing for Salesforce to reject.
	conditions, err := collectConditions(inputs)
	if err != nil {
		return nil, err
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// The LIMIT clause is only applied on a single-page run. With Return All on,
	// the cap comes from the pagination loop instead, so a LIMIT here would
	// truncate the very thing the operator asked to fetch in full.
	//
	// Typed rather than plain BuildQuery: whether a filter value is quoted
	// depends on the FIELD's Salesforce type, so "AnnualRevenue > 250000" needs
	// a bare number while "PostalCode = 12345" needs quotes. One cached describe
	// resolves that; if the connected user cannot read Lead's metadata the
	// builder falls back to the value-only heuristic rather than failing.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"Lead",
		salesforce.OptionalString("fields", inputs),
		conditions,
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
		// Stopping at the cap is not a failure, but it must be visible: the
		// operator is holding a partial answer and needs to know the rest is
		// still there behind the returned page URL.
		out["tool_result"] = fmt.Sprintf("Fetched %d lead(s) across %d page(s); stopped at the %d-page safety limit — more remain", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d lead(s) across %d page(s)", len(records), pages)
	case nextURL != "":
		out["tool_result"] = fmt.Sprintf("Found %d lead(s) of %d matching — turn on Return All to fetch the rest", len(records), totalSize)
	default:
		// Salesforce reports totalSize as the PAGE size once a LIMIT is applied
		// and sets done:true with no cursor, so a capped list is otherwise
		// indistinguishable from a complete one.
		out["tool_result"] = fmt.Sprintf("Found %d lead(s)%s", len(records), salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}

// collectConditions merges the simple one-row filter with the More Filters
// array. The simple row goes first so the WHERE clause reads in the order the
// operator filled the form in.
func collectConditions(inputs []*core.Connection) ([]salesforce.Condition, error) {
	field := salesforce.OptionalString("filter_field", inputs)
	value := salesforce.OptionalString("filter_value", inputs)

	// A value with nothing to compare it against is a half-filled form, not a
	// query. Silently ignoring it would return every lead in the org and look
	// as though the filter simply did not work.
	if field == "" && value != "" {
		return nil, fmt.Errorf("filter_value is set but filter_field is empty — choose the field to filter on, or clear the value")
	}

	var conditions []salesforce.Condition
	if field != "" {
		conditions = append(conditions, salesforce.Condition{
			Field:    field,
			Operator: salesforce.OptionalString("filter_operator", inputs),
			Value:    value,
		})
	}

	more, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	return append(conditions, more...), nil
}
