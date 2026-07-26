package crm_salesforce_account_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Accounts"
	Description  = "List accounts from Salesforce, optionally narrowed by a filter such as Industry or Billing City. Turn on Return All to fetch every match, page by page."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Id, Name, Phone, BillingCity (blank returns Id, Name, Type and Last Modified)"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "Industry"},
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
			{Name: "Matches pattern (use % as a wildcard)", Value: "LIKE"},
			{Name: "Does not match pattern", Value: "NOT LIKE"},
			{Name: "Is one of (comma-separated)", Value: "IN"},
			{Name: "Is not one of (comma-separated)", Value: "NOT IN"},
		},
	},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "Manufacturing (or TODAY, LAST_N_DAYS:7 for date fields)"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"NumberOfEmployees","operator":">","value":"250"}]`},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "Name ASC, or LastModifiedDate DESC"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted (records in the Recycle Bin)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Accounts"},
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

	// Two ways in to the same WHERE clause: the plain-English three-box filter
	// covers the common single condition, and the JSON list handles the rest.
	// Both go through the shared WHERE builder, which validates the field name
	// and operator and escapes the value — SOQL has no bind variables, so this
	// is the only thing between a filter value and an injected query.
	conditions, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	if field := salesforce.OptionalString("filter_field", inputs); field != "" {
		conditions = append(conditions, salesforce.Condition{
			Field:    field,
			Operator: salesforce.OptionalString("filter_operator", inputs),
			Value:    salesforce.OptionalString("filter_value", inputs),
		})
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	pageSize := salesforce.ClampLimit(limit, limitSet)

	// The LIMIT clause is only appended when NOT returning all: with Return All
	// on, the limit would cap the whole run rather than a page.
	//
	// BuildQueryTyped, not BuildQuery: whether a filter value is quoted depends
	// on the FIELD's Salesforce type, so "NumberOfEmployees > 250" only works if
	// the literal is bare. It resolves that from one cached describe call, and
	// falls back to the value-only heuristic if describe is denied.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"Account",
		salesforce.OptionalString("fields", inputs),
		conditions,
		salesforce.OptionalBool("match_any_filter", inputs),
		salesforce.OptionalString("order_by", inputs),
		pageSize,
		!returnAll,
	)
	if err != nil {
		// A bad field name, operator or sort direction is a configuration
		// mistake caught before the request is ever made.
		return nil, err
	}

	includeDeleted := salesforce.OptionalBool("include_deleted", inputs)
	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, includeDeleted)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	summary := fmt.Sprintf("Found %d account(s)", len(records))
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		summary = fmt.Sprintf("Fetched %d account(s) across %d page(s); stopped at the %d-page safety cap — narrow the filter to see the rest", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		summary = fmt.Sprintf("Fetched all %d account(s) across %d page(s)", len(records), pages)
	case nextURL != "":
		summary = fmt.Sprintf("Found %d account(s) of %d matching — turn on Return All to fetch the rest", len(records), totalSize)
	}
	return salesforce.ListResult(records, nextURL, totalSize, summary), nil
}
