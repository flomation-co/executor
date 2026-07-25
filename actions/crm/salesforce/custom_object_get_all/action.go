package crm_salesforce_custom_object_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Custom Object Records"
	Description  = "List records from one of your organisation's own Salesforce objects, narrowed by a filter and sorted however you like. Turn on Return All to page through every match."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// The filter is offered twice on purpose. One field / comparison / value covers
// the overwhelming majority of real flows ("Status is Open") without asking a
// front-of-house operator to write JSON; More Filters is there for the minority
// that need two or three conditions. Both go through the same validated builder,
// so neither is a way to hand-write a query.
var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "custom_object", Type: core.ConnectionTypeString, Label: "Custom Object", Placeholder: "Invoice__c — the object's API name, which almost always ends in __c", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. Name,Amount__c,Status__c (blank returns Id, Name and Last Modified)"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "Status__c — leave blank to return everything"},
	{
		Name:  "filter_operator",
		Type:  core.ConnectionTypeString,
		Label: "Filter Comparison",
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "="},
			{Name: "Does Not Equal", Value: "!="},
			{Name: "Less Than", Value: "<"},
			{Name: "Less Than Or Equal To", Value: "<="},
			{Name: "Greater Than", Value: ">"},
			{Name: "Greater Than Or Equal To", Value: ">="},
			{Name: "Contains (LIKE)", Value: "LIKE"},
			{Name: "Does Not Contain (NOT LIKE)", Value: "NOT LIKE"},
			{Name: "Is Any Of (IN)", Value: "IN"},
			{Name: "Is None Of (NOT IN)", Value: "NOT IN"},
		},
		Placeholder: "Equals",
	},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "Open — dates also accept TODAY, THIS_MONTH or LAST_N_DAYS:7; Is Any Of takes a comma-separated list"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"Amount__c","operator":">","value":"10000"}] — extra conditions on top of the one above`},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "CreatedDate DESC — a field name, optionally with ASC or DESC"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (page through every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 records (max 2000); ignored when Return All is on"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Records In The Recycle Bin"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Records"},
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

	object, err := salesforce.RequiredString("custom_object", inputs)
	if err != nil {
		return nil, err
	}
	object, err = salesforce.ValidateSOQLObjectName(object)
	if err != nil {
		return nil, err
	}

	// Simple filter first so it reads left-to-right in the query, then anything
	// from the JSON input. An empty Filter Field means "no condition at all"
	// rather than an error — listing everything is a legitimate ask.
	conditions := []salesforce.Condition{}
	if field := salesforce.OptionalString("filter_field", inputs); field != "" {
		conditions = append(conditions, salesforce.Condition{
			Field:    field,
			Operator: salesforce.OptionalString("filter_operator", inputs),
			Value:    salesforce.OptionalString("filter_value", inputs),
		})
	}
	more, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, more...)

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// Every identifier and operator below is whitelist-validated and every value
	// escaped inside the query builder — it is the injection boundary, which is
	// why the query is never assembled here. A failure from it is always a bad
	// field name, comparison or sort, i.e. a configuration mistake.
	//
	// BuildQueryTyped, not BuildQuery: the filter values are operator-supplied
	// and a custom object's fields are whatever the org made them, so only
	// Salesforce can say whether a literal needs quoting (Amount__c > 10000 is
	// valid, Amount__c > '10000' is INVALID_FIELD). One cached describe answers
	// that; if the connected user cannot describe the object the query is built
	// with the untyped heuristic instead of failing.
	//
	// Leaving Fields blank falls back to Id, Name and LastModifiedDate rather
	// than the Id + LastModifiedDate pair n8n returns, so a first run shows the
	// operator something they recognise.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		object,
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
		// The page cap exists so one flow cannot spend the org's whole daily API
		// allowance, which is shared with every other system touching Salesforce.
		out["tool_result"] = fmt.Sprintf("Fetched %d %s record(s) across %d page(s), then stopped at the %d-page safety limit — add a filter to narrow the results", len(records), object, pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d %s record(s) across %d page(s)", len(records), object, pages)
	case nextURL != "":
		out["tool_result"] = fmt.Sprintf("Found %d %s record(s) of %d matching — turn on Return All to fetch the rest", len(records), object, totalSize)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d %s record(s)", len(records), object)
	}
	return out, nil
}
