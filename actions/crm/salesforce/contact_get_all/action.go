// Package crm_salesforce_contact_get_all lists Contacts.
//
// There is no REST "list contacts" endpoint in Salesforce — listing means
// running a SOQL query — so this action builds one from plain-English inputs
// (pick fields, pick a filter, pick a sort) and never asks the operator to write
// query text. Every identifier is whitelist-validated and every value escaped
// and quoted by the shared query builder, which is the injection boundary: a
// company called "O'Brien Ltd" is a filter value, not a syntax error.
package crm_salesforce_contact_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Contacts"
	Description  = "List contacts, optionally filtered and sorted. Turn on Return All to fetch every match, or set a limit for a single page. No query writing needed."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},

	// Left blank this returns Id, FirstName, LastName, Email and
	// LastModifiedDate — Salesforce's query endpoint has no "all fields" form,
	// so a projection is always required and a useful default beats an ID-only
	// one.
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "FirstName, LastName, Email, Phone — blank for the usual few"},

	// The simple one-condition filter, which covers the overwhelming majority of
	// real uses ("contacts at this account", "email is this").
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "Email, AccountId, LastModifiedDate…"},
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
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "jane.smith@acme.com — or TODAY, LAST_N_DAYS:7 for dates"},

	// The advanced escape hatch: several conditions at once. Anything set here
	// is combined with the simple filter above.
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"Department","operator":"=","value":"Finance"}]`},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},

	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "LastModifiedDate DESC, LastName ASC"},

	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},

	// Deleted contacts sit in the Recycle Bin and are invisible to an ordinary
	// query, which is why people think records have vanished. This switches to
	// the queryAll endpoint so they come back with an IsDeleted flag.
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted and Archived"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Contacts"},
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

	conditions, err := collectConditions(inputs)
	if err != nil {
		return nil, err
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// The LIMIT clause is what caps a single page; with Return All on it is
	// omitted entirely and the nextRecordsUrl chain is followed instead.
	//
	// BuildQueryTyped, not BuildQuery: every filter value here is operator
	// supplied, and whether a SOQL literal is quoted depends on the FIELD's type
	// (a number field rejects '10', a text field rejects 10). One cached describe
	// resolves that; if describe is denied the query falls back to the untyped
	// heuristic rather than failing.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"Contact",
		salesforce.OptionalString("fields", inputs),
		conditions,
		salesforce.OptionalBool("match_any_filter", inputs),
		salesforce.OptionalString("order_by", inputs),
		salesforce.ClampLimit(limit, limitSet),
		!returnAll,
	)
	if err != nil {
		// A bad field name, comparison or sort direction is a configuration
		// mistake caught before anything reaches Salesforce.
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, salesforce.OptionalBool("include_deleted", inputs))
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d contact(s) across %d page(s); stopped at the %d-page safety cap — narrow the filter or use the returned page URL to continue", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d contact(s) across %d page(s)", len(records), pages)
	case nextURL != "":
		// Salesforce reports totalSize as the size of THIS page once a LIMIT is
		// applied, so it is not a count of everything that matched — say there
		// is more rather than quoting a number that means something else.
		out["tool_result"] = fmt.Sprintf("Found %d contact(s); more are available — raise the limit or turn on Return All", len(records))
	default:
		out["tool_result"] = fmt.Sprintf("Found %d contact(s)", len(records))
	}
	return out, nil
}

// collectConditions merges the simple one-line filter with the advanced JSON
// list. Both feed the same validated builder — there is deliberately no path
// that puts operator text into the query unchecked.
func collectConditions(inputs []*core.Connection) ([]salesforce.Condition, error) {
	var conditions []salesforce.Condition

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
