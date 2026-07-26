package crm_salesforce_opportunity_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Opportunities"
	Description  = "List deals from Salesforce, optionally filtered - everything closing this month, every deal on one account, everything still open. Turn on Return All to fetch every match."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// defaultFields is the projection used when the operator picks no fields.
//
// n8n's equivalent default omits Name, StageName and CloseDate, which is why its
// users regularly report that Salesforce "lost" their opportunity names. The
// three things anyone looking at a pipeline actually wants are the name, the
// stage and the close date, so they lead here.
const defaultFields = "Id,Name,AccountId,Amount,StageName,CloseDate,Probability,Type,OwnerId,LastModifiedDate"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Name,Amount,StageName,CloseDate - leave blank for the usual pipeline fields"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "StageName - the field to filter on"},
	{
		Name:        "filter_operator",
		Type:        core.ConnectionTypeString,
		Label:       "Comparison",
		Placeholder: "How to compare the field to the value (defaults to equals)",
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "="},
			{Name: "Does Not Equal", Value: "!="},
			{Name: "Less Than", Value: "<"},
			{Name: "Less Than Or Equal To", Value: "<="},
			{Name: "Greater Than", Value: ">"},
			{Name: "Greater Than Or Equal To", Value: ">="},
			{Name: "Contains (use % as the wildcard)", Value: "LIKE"},
			{Name: "Does Not Contain (use % as the wildcard)", Value: "NOT LIKE"},
			{Name: "Is One Of (IN)", Value: "IN"},
			{Name: "Is Not One Of (NOT IN)", Value: "NOT IN"},
		},
	},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "Closed Won - or a date word like THIS_MONTH, or Closed Won,Closed Lost for Is One Of — stage names must match your org's Opportunity Stage list"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: "[{\"field\":\"Amount\",\"operator\":\">\",\"value\":\"10000\"},{\"field\":\"IsClosed\",\"operator\":\"=\",\"value\":\"false\"}] - extra filters"},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "CloseDate ASC - or Amount DESC"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted Deals (Recycle Bin)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Opportunities"},
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

	fields := salesforce.OptionalString("fields", inputs)
	if fields == "" {
		fields = defaultFields
	}

	// The simple field/comparison/value trio covers the overwhelming majority of
	// filters and needs no JSON. filter_conditions is the escape hatch for
	// anything more involved, and both feed the same validated WHERE builder —
	// nothing here concatenates a query by hand.
	var conditions []salesforce.Condition
	if field := salesforce.OptionalString("filter_field", inputs); field != "" {
		conditions = append(conditions, salesforce.Condition{
			Field:    field,
			Operator: salesforce.OptionalString("filter_operator", inputs),
			Value:    salesforce.OptionalString("filter_value", inputs),
		})
	}
	extra, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, extra...)

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	clamped := salesforce.ClampLimit(limit, limitSet)

	// Salesforce has no separate page-size parameter for SOQL: the LIMIT is part
	// of the statement, so it is only applied when the operator wants one page.
	//
	// BuildQueryTyped resolves each filter value against the field's real type
	// via one cached describe, so "Amount > 10000" renders bare and "IsClosed =
	// false" renders as a boolean — the two comparisons a pipeline report is
	// built from, and both hard errors under the value-only heuristic.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"Opportunity",
		fields,
		conditions,
		salesforce.OptionalBool("match_any_filter", inputs),
		salesforce.OptionalString("order_by", inputs),
		clamped,
		!returnAll,
	)
	if err != nil {
		// A bad field name or comparison is a configuration mistake — fail hard
		// rather than handing Salesforce a query it will only reject.
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(
		instanceURL, token, soql, returnAll, salesforce.OptionalBool("include_deleted", inputs),
	)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	noun := plural(len(records))
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d %s across %d page(s); stopped at the %d-page safety cap — narrow the filter or resume from the returned next page URL", len(records), noun, pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d %s across %d page(s)", len(records), noun, pages)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d %s%s", len(records), noun, salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}

// plural picks the right noun for a count, so the summary an operator reads in
// the execution view is a sentence rather than "1 opportunities".
func plural(n int) string {
	if n == 1 {
		return "opportunity"
	}
	return "opportunities"
}
