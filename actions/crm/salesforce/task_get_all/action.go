package crm_salesforce_task_get_all

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Tasks"
	Description  = "Find Salesforce tasks — everything due this week, everything still open for one rep, everything logged against a customer. Fill in the simple filters, or add your own for anything more involved."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields To Return", Placeholder: "Id, Subject, Status, ActivityDate — leave blank for the usual ones"},
	{Name: "task_status", Type: core.ConnectionTypeString, Label: "Status Is", Placeholder: "Not Started"},
	{Name: "task_priority", Type: core.ConnectionTypeString, Label: "Priority Is", Placeholder: "High"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Assigned To", Placeholder: "Salesforce user ID of the owner"},
	{Name: "who_id", Type: core.ConnectionTypeString, Label: "About Contact or Lead", Placeholder: "Record ID of the person the task is about"},
	{Name: "what_id", Type: core.ConnectionTypeString, Label: "Related Record", Placeholder: "Record ID of the account, opportunity or case"},
	{Name: "due_from", Type: core.ConnectionTypeDateTime, Label: "Due On or After", Placeholder: "Only tasks due from this day onwards"},
	{Name: "due_to", Type: core.ConnectionTypeDateTime, Label: "Due On or Before", Placeholder: "Only tasks due up to this day"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "Any other field to filter on, e.g. Subject"},
	{Name: "filter_operator", Type: core.ConnectionTypeString, Label: "Filter Comparison", Placeholder: "= (also !=, <, <=, >, >=, LIKE, IN)"},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "Renewal call — or a date shortcut like TODAY or LAST_N_DAYS:7"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"IsClosed","operator":"=","value":"false"}]`},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "ActivityDate ASC"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000) — ignored when Return All is on"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted and Archived Tasks"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Tasks"},
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

	// Every filter — the plain-English boxes and the JSON escape hatch alike —
	// becomes a Condition and goes through BuildQueryTyped, which is the one
	// place identifiers are whitelisted and values are escaped — and which asks
	// Salesforce what each field actually is, so a number compares as a number.
	// Nothing here builds SOQL by hand.
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
	conditions = appendEqualsFilter(conditions, "Status", salesforce.OptionalString("task_status", inputs))
	conditions = appendEqualsFilter(conditions, "Priority", salesforce.OptionalString("task_priority", inputs))
	conditions = appendEqualsFilter(conditions, "OwnerId", salesforce.OptionalString("owner_id", inputs))
	conditions = appendEqualsFilter(conditions, "WhoId", salesforce.OptionalString("who_id", inputs))
	conditions = appendEqualsFilter(conditions, "WhatId", salesforce.OptionalString("what_id", inputs))
	// ActivityDate is a Date field: comparing it to a full timestamp is a
	// malformed query, so the date picker's value is trimmed to the day.
	if from := dateOnly(salesforce.OptionalString("due_from", inputs)); from != "" {
		conditions = append(conditions, salesforce.Condition{Field: "ActivityDate", Operator: ">=", Value: from})
	}
	if to := dateOnly(salesforce.OptionalString("due_to", inputs)); to != "" {
		conditions = append(conditions, salesforce.Condition{Field: "ActivityDate", Operator: "<=", Value: to})
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	soql, err := salesforce.BuildQueryTyped(
		instanceURL, token,
		"Task",
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
		out["tool_result"] = fmt.Sprintf("Fetched %d task(s) across %d page(s); stopped at the %d-page safety cap — narrow the filters to see the rest", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d task(s) across %d page(s)", len(records), pages)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d task(s)", len(records))
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

// dateOnly trims a date-picker value down to YYYY-MM-DD, the only form a
// Salesforce Date field can be compared against.
func dateOnly(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, "T"); i == 10 {
		return v[:10]
	}
	return v
}
