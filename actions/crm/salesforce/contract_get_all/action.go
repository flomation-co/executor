package crm_salesforce_contract_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Contracts"
	Description  = "List contracts from Salesforce - everything ending in the next 60 days, every contract on one account, everything still in Draft. Pair it with a schedule and an email step and you have a renewals reminder. Turn on Return All to fetch every match instead of one page."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// defaultFields is the projection used when the operator picks no fields.
//
// It CANNOT be left to the shared default. Contract has no Name field at all
// (verified live), so the generic "Id,Name,LastModifiedDate" fallback is a hard
// INVALID_FIELD on this object. ContractNumber is the identifier a person
// recognises, and EndDate is the whole point of a renewals list.
const defaultFields = "Id,ContractNumber,AccountId,Status,StartDate,EndDate,ContractTerm,OwnerId,CustomerSignedDate,LastModifiedDate"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "ContractNumber,Status,StartDate,EndDate - leave blank for the usual contract fields"},
	{Name: "ends_on_or_after", Type: core.ConnectionTypeString, Label: "Ends On Or After", Placeholder: "TODAY - or a date like 2026-08-01; always applies, even with Match ANY on"},
	{Name: "ends_on_or_before", Type: core.ConnectionTypeString, Label: "Ends On Or Before", Placeholder: "NEXT_N_DAYS:60 - the renewals window; always applies, even with Match ANY on"},
	{Name: "contract_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Activated, Draft or In Approval Process"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Customer (Account)", Placeholder: "0015f00000AbCdEAAV - only contracts with this account"},
	{Name: "owner_id", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "0055f00000AbCdEAAV - only contracts owned by this Salesforce user"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "One more field to filter on, e.g. ContractTerm or Renewal_Owner__c"},
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
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "What to compare against, e.g. 12, THIS_YEAR, 2026-09-01"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: "[{\"field\":\"ContractTerm\",\"operator\":\">=\",\"value\":\"12\"},{\"field\":\"Status\",\"operator\":\"=\",\"value\":\"Activated\"}]"},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all", Placeholder: "Off: a contract has to match every filter. On: matching any one of them is enough. Ends On Or After / Before always apply either way"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "EndDate ASC - or ContractNumber DESC"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 by default, up to 2000 - ignored when Return All is on"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Tick to keep fetching pages until every matching contract has been collected"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted", Placeholder: "Tick to include contracts sitting in the Salesforce Recycle Bin"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Contracts"},
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

	filters, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}

	// Two lists, not one. "Ends On Or After" and "Ends On Or Before" are the two
	// ends of a RANGE, and letting the Match ANY toggle OR them together does not
	// merely widen the result — it is a tautology that returns every contract in
	// the org (see scoped_query.go). They are scope and are always ANDed. Status,
	// account and owner are ordinary filters: "Draft OR owned by Priya" is a
	// coherent thing for an operator to ask for.
	//
	// EndDate is READ-ONLY on Contract — Salesforce derives it from the start date
	// plus the term, less a day (verified live: 2026-08-01 + 12 months =
	// 2027-07-31, and moving the term to 24 moved it to 2028-07-31). It is not a
	// formula field in the describe sense (calculated is false); it is simply
	// neither createable nor updateable, and there is nothing in Setup to look at.
	// Salesforce still reports it filterable and sortable, which is what makes the
	// renewals window possible at all.
	var scope []salesforce.Condition
	if v := salesforce.OptionalString("ends_on_or_after", inputs); v != "" {
		scope = append(scope, salesforce.Condition{Field: "EndDate", Operator: ">=", Value: v})
	}
	if v := salesforce.OptionalString("ends_on_or_before", inputs); v != "" {
		scope = append(scope, salesforce.Condition{Field: "EndDate", Operator: "<=", Value: v})
	}
	if v := salesforce.OptionalString("contract_status", inputs); v != "" {
		filters = append(filters, salesforce.Condition{Field: "Status", Operator: "=", Value: v})
	}
	// Both lookup boxes are checked for ID SHAPE before they go anywhere near a
	// WHERE clause. Salesforce answers an ID field compared against a name or an
	// email address with a raw parser dump — the SOQL fragment, a caret, a
	// row/column position and INVALID_QUERY_FILTER_OPERATOR, a code that reads as
	// though the operator's Filter Comparison is at fault (verified live with
	// account_id="Edge Communications" and owner_id="priya@example.com"). An
	// upstream step that resolved to a name is the ordinary way to get here.
	if v := salesforce.OptionalString("account_id", inputs); v != "" {
		if err := salesforce.ValidateRecordID(v); err != nil {
			return nil, fmt.Errorf("Customer (Account) — %w. Pick the account from the list, or pass the ID from an earlier Salesforce step rather than the company name", err)
		}
		filters = append(filters, salesforce.Condition{Field: "AccountId", Operator: "=", Value: v})
	}
	if v := salesforce.OptionalString("owner_id", inputs); v != "" {
		if err := salesforce.ValidateRecordID(v); err != nil {
			return nil, fmt.Errorf("Owner — %w. Pick the user from the list, or pass their Salesforce user ID rather than their name or email address", err)
		}
		filters = append(filters, salesforce.Condition{Field: "OwnerId", Operator: "=", Value: v})
	}
	if field := salesforce.OptionalString("filter_field", inputs); field != "" {
		filters = append(filters, salesforce.Condition{
			Field:    field,
			Operator: salesforce.OptionalString("filter_operator", inputs),
			Value:    salesforce.OptionalString("filter_value", inputs),
		})
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// The typed builder, not the untyped one: whether Salesforce wants a value
	// quoted depends on the FIELD, and Contract has both kinds side by side —
	// ContractTerm > 12 is valid where ContractTerm > '12' is INVALID_FIELD, and
	// EndDate <= NEXT_N_DAYS:60 has to stay an unquoted keyword. One cached
	// describe resolves all of it, and a denied describe degrades rather than
	// failing the run.
	//
	// applyLimit is off for Return All: a LIMIT clause would cut the result short
	// at exactly the point the operator asked for everything.
	soql, err := salesforce.BuildScopedQueryTyped(
		instanceURL,
		token,
		"Contract",
		fields,
		scope,
		filters,
		salesforce.OptionalBool("match_any_filter", inputs),
		salesforce.OptionalString("order_by", inputs),
		salesforce.ClampLimit(limit, limitSet),
		!returnAll,
	)
	if err != nil {
		// A bad field name or comparison is a configuration mistake — fail hard
		// rather than handing Salesforce a query it will only reject.
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, salesforce.OptionalBool("include_deleted", inputs))
	if err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf("contracts are not available in your Salesforce org — an administrator can switch them on under Setup ▸ Contract Settings, and some Salesforce editions do not include them at all (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d contract(s) across %d page(s); stopped at the %d-page safety cap — narrow the filters to see the rest", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d contract(s) across %d page(s)", len(records), pages)
	default:
		// Deliberately not "x of y": Salesforce reports totalSize as the PAGE size
		// once a LIMIT is applied and sets done:true with no cursor, so a capped
		// list is otherwise indistinguishable from a complete one.
		out["tool_result"] = fmt.Sprintf("Found %d contract(s)%s", len(records), salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}
