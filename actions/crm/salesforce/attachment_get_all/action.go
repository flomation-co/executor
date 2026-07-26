// Package crm_salesforce_attachment_get_all lists Classic Attachments.
//
// There is no REST list endpoint for attachments — listing means running a SOQL
// query — so this builds one from operator-friendly parts rather than asking a
// receptionist to write SOQL. Every identifier is whitelist-validated and every
// value escaped by the shared builder, which is the injection boundary for the
// whole node.
//
// Two filter styles are offered on purpose. One field, one comparison, one
// value covers the overwhelming case ("everything on this record") in three
// boxes; the JSON conditions input is there for the rarer multi-clause filter
// without making the simple case look complicated. n8n offers only the second
// and only ever ANDs it — this can OR, and can sort.
package crm_salesforce_attachment_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Attachments (Classic)"
	Description  = "List Classic attachments, optionally filtered and sorted — for example every attachment on one record, or everything added since yesterday."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},

	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},

	// Blank gives Id, Name and LastModifiedDate. Body is deliberately not in
	// the default list: it comes back as a URL path, not the file, and putting
	// it in front of people is what makes them wire it into an email.
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields to Return", Placeholder: "Leave blank for the usual ones, or e.g. Id,Name,ContentType,BodyLength,ParentId"},

	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter On Field", Placeholder: "ParentId"},
	{
		Name:        "filter_operator",
		Type:        core.ConnectionTypeString,
		Label:       "Comparison",
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
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "0015f00000XyzAAB — for Matches Pattern use % as the wildcard, e.g. %invoice%"},

	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: `[{"field":"BodyLength","operator":">","value":"1000000"}]`},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},

	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "CreatedDate DESC, Name ASC"},

	// /queryAll rather than /query. Recycle Bin records are invisible to a
	// normal query, which is why attachments look like they have "vanished"
	// when someone has deleted them.
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted Attachments (Recycle Bin)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Attachments"},
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
	conditions, err := buildConditions(inputs)
	if err != nil {
		return nil, err
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// The LIMIT clause is only applied on a single-page run. With Return All
	// on, the cap comes from the pagination loop instead, so a LIMIT here would
	// truncate the very thing the operator asked to fetch in full.
	//
	// BuildQueryTyped, not BuildQuery: whether a filter value is quoted depends
	// on the FIELD's Salesforce type, so "BodyLength > 1000000" only works if
	// the literal is bare. It resolves that from one cached describe call, and
	// falls back to the value-only heuristic if describe is denied.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"Attachment",
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
		out["tool_result"] = fmt.Sprintf("Fetched %d attachment(s) across %d page(s); stopped at the %d-page safety limit — more remain", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d attachment(s) across %d page(s)", len(records), pages)
	case nextURL != "":
		out["tool_result"] = fmt.Sprintf("Found %d attachment(s) of %d matching — turn on Return All to fetch the rest", len(records), totalSize)
	default:
		// Salesforce reports totalSize as the PAGE size once a LIMIT is applied
		// and sets done:true with no cursor, so a capped list is otherwise
		// indistinguishable from a complete one.
		out["tool_result"] = fmt.Sprintf("Found %d attachment(s)%s", len(records), salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}

// buildConditions merges the simple three-box filter with the JSON conditions
// list. The simple one goes first so the generated query reads the way the
// operator filled the form in.
//
// A comparison with no field is ignored rather than erroring: leaving the
// dropdown on its default while filling nothing else in is a normal thing to
// do, and it means "no filter".
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
