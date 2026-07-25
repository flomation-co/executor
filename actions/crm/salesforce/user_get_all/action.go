// Package crm_salesforce_user_get_all lists the people in a Salesforce org.
//
// User has no filterable REST list endpoint — like every other object, listing
// means running SOQL — so this builds the query from operator-friendly parts and
// leaves the shared builder to validate every identifier and escape every value.
//
// The one filter that matters here is "active users only". A Salesforce user is
// never deleted, only deactivated, so an org's User table accumulates every
// person who has ever worked there; a rota or approval flow that iterates it
// unfiltered will happily assign work to someone who left three years ago. That
// is why it is a tick box rather than something to express in the filter JSON.
//
// Deleted records are deliberately not an option here (no /queryAll toggle, as
// the record list actions carry): there is no such thing as a deleted User to
// find in the Recycle Bin.
package crm_salesforce_user_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Users"
	Description  = "List the people in your Salesforce org, with optional filters such as active users only. Enable Return All to fetch every match."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields to Return", Placeholder: "Leave blank for the usual ones, or list them: Name, Email, IsActive, Title"},
	{Name: "active_only", Type: core.ConnectionTypeBoolean, Label: "Active Users Only", Placeholder: "Hide people whose Salesforce login has been switched off"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter On Field", Placeholder: "Department — the field to filter by (optional)"},
	{
		Name:        "filter_operator",
		Type:        core.ConnectionTypeString,
		Label:       "Filter Comparison",
		Placeholder: "Equals",
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "="},
			{Name: "Does Not Equal", Value: "!="},
			{Name: "Less Than", Value: "<"},
			{Name: "Less Than Or Equal To", Value: "<="},
			{Name: "Greater Than", Value: ">"},
			{Name: "Greater Than Or Equal To", Value: ">="},
			{Name: "Matches Pattern", Value: "LIKE"},
			{Name: "Does Not Match Pattern", Value: "NOT LIKE"},
			{Name: "Is One Of", Value: "IN"},
			{Name: "Is Not One Of", Value: "NOT IN"},
		},
	},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "Sales — for Matches Pattern use % as the wildcard (%sales%); for Is One Of, separate values with commas"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"Department","operator":"=","value":"Sales"},{"field":"Title","operator":"LIKE","value":"%Manager%"}]`},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all", Placeholder: "On, any one filter is enough to match — including Active Users Only"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "Name ASC — or LastLoginDate DESC"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 by default (max 2000); ignored when Return All is on"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Users"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total_size", Type: core.ConnectionTypeInteger, Label: "Total Matching"},
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

	conditions, err := buildConditions(inputs)
	if err != nil {
		return nil, err
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// The limit is applied as a SOQL LIMIT clause, not a REST parameter —
	// Salesforce has no page-size parameter on /query. With Return All on there
	// is deliberately no LIMIT so the nextRecordsUrl chain runs to the end.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL, token,
		"User",
		salesforce.OptionalString("fields", inputs),
		conditions,
		salesforce.OptionalBool("match_any_filter", inputs),
		salesforce.OptionalString("order_by", inputs),
		salesforce.ClampLimit(limit, limitSet),
		!returnAll,
	)
	if err != nil {
		// Every failure BuildQueryTyped can produce is a rejected identifier,
		// operator, sort direction or a value that does not suit its field's type
		// — an editing mistake, not a Salesforce problem. A describe the connected
		// user cannot run is not a failure: it degrades to the untyped path.
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, false)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d user(s) across %d page(s); stopped at the %d-page safety cap — narrow the filters to see the rest", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d user(s) across %d page(s)", len(records), pages)
	case nextURL != "":
		out["tool_result"] = fmt.Sprintf("Found %d user(s) of %d matching — turn on Return All to fetch the rest", len(records), totalSize)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d user(s)", len(records))
	}
	return out, nil
}

// buildConditions assembles the WHERE terms from the three filter styles this
// action offers, in the order they are shown in the editor.
//
// They deliberately stack rather than override each other: "active users only"
// plus a department filter is the single most common request, and making the
// operator express that as JSON would defeat the point of the tick box.
//
// Note the interaction with "Match ANY filter instead of all": the WHERE builder
// joins every term with one connective (SOQL bracket grouping is not exposed
// here), so turning it on ORs the Active Users Only term along with everything
// else. The input's help text says so rather than the code quietly
// special-casing it.
func buildConditions(inputs []*core.Connection) ([]salesforce.Condition, error) {
	var conditions []salesforce.Condition

	if salesforce.OptionalBool("active_only", inputs) {
		conditions = append(conditions, salesforce.Condition{Field: "IsActive", Operator: "=", Value: "true"})
	}

	if field := salesforce.OptionalString("filter_field", inputs); field != "" {
		operator := salesforce.OptionalString("filter_operator", inputs)
		if operator == "" {
			operator = "="
		}
		conditions = append(conditions, salesforce.Condition{
			Field:    field,
			Operator: operator,
			Value:    salesforce.OptionalString("filter_value", inputs),
		})
	}

	extra, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	return append(conditions, extra...), nil
}
