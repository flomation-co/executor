// Package crm_salesforce_case_get_all lists support Cases.
//
// Salesforce has no "list cases" REST endpoint — listing anything means writing
// SOQL, which is precisely the wall a non-technical operator hits. This action
// builds the query for them: pick the columns, pick a filter, done. Everything
// the operator types is validated or escaped by the shared builder
// (BuildQuery/BuildWhere/SOQLValue), which is the injection boundary for the
// whole node; nothing here assembles SOQL by hand.
//
// Two parity-pluses over n8n, both cheap: filters can be combined with OR (n8n
// is AND-only) and the full operator set is exposed (n8n's UI offers five of
// the twelve its own validator accepts, so LIKE and IN are unreachable).
package crm_salesforce_case_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Cases"
	Description  = "List Salesforce cases, optionally filtered — open cases, cases for one account, cases raised this week. Enable Return All to fetch every match rather than a single page."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// caseListFields is the SELECT list used when the operator picks no columns.
//
// It is deliberately NOT salesforce.DefaultFields("Case"): that shared default
// omits CaseNumber, and a case list without the number the customer quotes on
// the phone is unusable at a front desk. The rest of the list is what a triage
// screen needs — who it is for, how urgent, who owns it, when it moved.
const caseListFields = "Id,CaseNumber,Subject,Status,Priority,Type,Origin,AccountId,ContactId,OwnerId,IsClosed,CreatedDate,LastModifiedDate"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank — taken from your connection", FromCredentialMeta: "instance_url"},

	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Leave blank for the usual case columns, or list them: CaseNumber,Subject,Status"},

	// The simple one-line filter covers the overwhelming majority of real uses
	// ("Status is New", "AccountId is this one"). filter_conditions below is the
	// escape hatch for anyone who needs several at once.
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter On", Placeholder: "Status — the field to filter by"},
	{
		Name:        "filter_operator",
		Type:        core.ConnectionTypeString,
		Label:       "Comparison",
		Placeholder: "Defaults to Equals",
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "="},
			{Name: "Does not equal", Value: "!="},
			{Name: "Less than", Value: "<"},
			{Name: "Less than or equal to", Value: "<="},
			{Name: "Greater than", Value: ">"},
			{Name: "Greater than or equal to", Value: ">="},
			{Name: "Contains (use % as the wildcard)", Value: "LIKE"},
			{Name: "Does not contain (use % as the wildcard)", Value: "NOT LIKE"},
			{Name: "Is one of (comma-separated)", Value: "IN"},
			{Name: "Is not one of (comma-separated)", Value: "NOT IN"},
		},
	},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "New — or a date shortcut such as TODAY or LAST_N_DAYS:7"},

	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"Priority","operator":"=","value":"High"},{"field":"IsClosed","operator":"=","value":"false"}]`},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},

	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "CreatedDate DESC — newest first"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 by default, up to 2000 (ignored when Return All is on)"},

	// Deleted cases sit in the Recycle Bin for 15 days and are invisible to a
	// normal query. This is the only way to see them, which is what makes it
	// worth an input rather than an assumption.
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted Cases"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Cases"},
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

	conditions, err := buildConditions(inputs)
	if err != nil {
		return nil, err
	}

	fields := salesforce.OptionalString("fields", inputs)
	if fields == "" {
		fields = caseListFields
	}
	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// The LIMIT clause is what caps a single page, so it must NOT be applied
	// when the operator asked for every match — a query carrying both LIMIT 50
	// and a follow-the-cursor loop would stop dead after fifty rows.
	//
	// BuildQueryTyped, not BuildQuery: every value in the WHERE clause here is
	// typed by the operator, and whether a SOQL literal is quoted depends on the
	// FIELD's type, not the value's. One cached describe of Case is what makes
	// "IsClosed = false" and "Priority = 'High'" both come out right; a describe
	// the connected user cannot run degrades to the old heuristic rather than
	// failing the action.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"Case",
		fields,
		conditions,
		salesforce.OptionalBool("match_any_filter", inputs),
		salesforce.OptionalString("order_by", inputs),
		salesforce.ClampLimit(limit, limitSet),
		!returnAll,
	)
	if err != nil {
		// An unusable field name, operator or sort direction is a configuration
		// mistake — fail hard so it is fixed rather than retried.
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, salesforce.OptionalBool("include_deleted", inputs))
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d case(s) across %d page(s); stopped at the %d-page safety cap — narrow the filter or re-run from the returned next page URL", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d case(s) across %d page(s)", len(records), pages)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d case(s)%s", len(records), salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}

// buildConditions merges the simple one-line filter with the advanced JSON
// filter list, in that order, so a flow can pin one condition in the simple
// fields and add the rest as data.
//
// A filter value on its own is ignored rather than guessed at: without a field
// name there is nothing to compare it to, and silently dropping it is far
// safer than inventing a column.
func buildConditions(inputs []*core.Connection) ([]salesforce.Condition, error) {
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
