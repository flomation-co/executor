package crm_salesforce_campaign_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Campaigns"
	Description  = "List your Salesforce campaigns — all of them, or just the ones you care about, such as active campaigns or everything with a status of In Progress. Turn on Return All to fetch every match instead of one page."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Leave blank for the usual ones, or list them: Name, Status, Type, StartDate, NumberOfResponses"},
	{Name: "active_only", Type: core.ConnectionTypeBoolean, Label: "Active Campaigns Only", Placeholder: "Tick to hide campaigns that have been switched off"},
	{Name: "campaign_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Planned, In Progress, Completed or Aborted"},
	{Name: "campaign_type", Type: core.ConnectionTypeString, Label: "Campaign Type", Placeholder: "Webinar, Conference, Trade Show, Email, Advertisement, Direct Mail, Other"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "One more field to filter on, e.g. StartDate or Region__c"},
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
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "What to compare against, e.g. THIS_MONTH, 2026-09-01, %Open Day%"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"BudgetedCost","operator":">","value":"5000"},{"field":"StartDate","operator":">=","value":"THIS_YEAR"}]`},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all", Placeholder: "Off: a campaign has to match every filter. On: matching any one of them is enough"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "StartDate DESC, Name ASC"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 by default, up to 2000 — ignored when Return All is on"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Tick to keep fetching pages until every matching campaign has been collected"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted", Placeholder: "Tick to include campaigns sitting in the Salesforce Recycle Bin"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Campaigns"},
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

	// Every filter — the plain-English tick boxes, the single field/comparison/
	// value row, and the JSON escape hatch — is funnelled into one Condition
	// list. BuildQueryTyped is then the single place a value can reach the SOQL
	// string, and it is the only place that has to be right about escaping.
	conditions, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	if salesforce.OptionalBool("active_only", inputs) {
		conditions = append(conditions, salesforce.Condition{Field: "IsActive", Operator: "=", Value: "true"})
	}
	if v := salesforce.OptionalString("campaign_status", inputs); v != "" {
		conditions = append(conditions, salesforce.Condition{Field: "Status", Operator: "=", Value: v})
	}
	if v := salesforce.OptionalString("campaign_type", inputs); v != "" {
		conditions = append(conditions, salesforce.Condition{Field: "Type", Operator: "=", Value: v})
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

	// applyLimit is off for Return All: a LIMIT clause would cut the result
	// short at exactly the point the operator asked for everything. Paging is
	// then bounded by Query's own MaxAllPages cap instead.
	//
	// BuildQueryTyped, not BuildQuery: every filter here carries an
	// operator-supplied value, and whether Salesforce wants that value quoted
	// depends on the field — BudgetedCost > 5000 is valid where
	// BudgetedCost > '5000' is INVALID_FIELD. It resolves that from one cached
	// describe and falls back to the untyped rendering if describe is denied.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"Campaign",
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
		out["tool_result"] = fmt.Sprintf("Fetched %d campaign(s) across %d page(s); stopped at the %d-page safety cap — narrow the filters to see the rest", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d campaign(s) across %d page(s)", len(records), pages)
	default:
		// Deliberately not "x of y": under a LIMIT clause Salesforce reports
		// totalSize as the size of the limited batch, so quoting it as a total
		// would tell the operator there are no more when there may well be.
		out["tool_result"] = fmt.Sprintf("Found %d campaign(s)", len(records))
	}
	return out, nil
}
