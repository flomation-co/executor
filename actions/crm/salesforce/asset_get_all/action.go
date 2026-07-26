package crm_salesforce_asset_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Assets"
	Description  = "List what customers own - everything on one account when they ring up, every unit of one product, everything whose warranty runs out in the next month. Turn on Return All to fetch every match instead of one page."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// defaultFields is the projection used when the operator picks no fields.
//
// The shared fallback would give back the name and nothing else, and the whole
// value of an asset list is the detail beside the name: what product it is, the
// serial number to quote, and the warranty date that decides whether the call is
// chargeable.
const defaultFields = "Id,Name,AccountId,ContactId,Product2Id,SerialNumber,Status,InstallDate,PurchaseDate,UsageEndDate,Price,Quantity,LastModifiedDate"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Name,SerialNumber,Status,UsageEndDate - leave blank for the usual asset fields"},
	{Name: "warranty_ends_on_or_after", Type: core.ConnectionTypeString, Label: "Warranty Ends On Or After", Placeholder: "TODAY - or a date like 2026-08-01; always applies, even with Match ANY on"},
	{Name: "warranty_ends_on_or_before", Type: core.ConnectionTypeString, Label: "Warranty Ends On Or Before", Placeholder: "NEXT_N_DAYS:30 - the expiring-warranty window; always applies, even with Match ANY on"},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Customer (Account)", Placeholder: "0015f00000AbCdEAAV - only assets belonging to this account"},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Customer (Contact)", Placeholder: "0035f00000XyZabAAF - only assets belonging to this contact"},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - only assets of this product"},
	{Name: "asset_status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "Installed, Shipped, Registered, Purchased or Obsolete"},
	{Name: "serial_number", Type: core.ConnectionTypeString, Label: "Serial Number", Placeholder: "SN-00042 - find the one unit by the number stamped on it"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "One more field to filter on, e.g. Price or Site_Code__c"},
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
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "What to compare against, e.g. 5000, LAST_N_DAYS:90, %Diesel%"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: "[{\"field\":\"Price\",\"operator\":\">\",\"value\":\"5000\"},{\"field\":\"IsCompetitorProduct\",\"operator\":\"=\",\"value\":\"false\"}]"},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all", Placeholder: "Off: an asset has to match every filter. On: matching any one of them is enough. Warranty Ends On Or After / Before always apply either way"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "UsageEndDate ASC - or InstallDate DESC"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 by default, up to 2000 - ignored when Return All is on"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Tick to keep fetching pages until every matching asset has been collected"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted", Placeholder: "Tick to include assets sitting in the Salesforce Recycle Bin"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Assets"},
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

	// Two lists, not one. The two warranty boxes are the ends of a RANGE, and
	// letting the Match ANY toggle OR them together does not merely widen the
	// result — it is a tautology that returns every asset in the org, including the
	// ones with no warranty date at all (see scoped_query.go).
	//
	// Verified live on this object, because the mechanism is not the obvious one:
	// neither half lets a null through on its own (`UsageEndDate >= TODAY` and
	// `UsageEndDate <= NEXT_N_DAYS:2000` each matched only the dated asset), but
	// ORing the two together matched the asset with UsageEndDate null as well.
	// Salesforce collapses the pair into an always-true predicate and stops
	// filtering on the field at all. So this is not a "wider window" bug an
	// operator could spot by reading the result — the warranty filter simply
	// vanishes. They are scope and are always ANDed.
	//
	// Account, contact, product, status and serial number are ordinary filters and
	// stay under the toggle: "on this account OR with this serial number" is a
	// coherent thing for someone on a support call to ask for.
	var scope []salesforce.Condition
	if v := salesforce.OptionalString("warranty_ends_on_or_after", inputs); v != "" {
		scope = append(scope, salesforce.Condition{Field: "UsageEndDate", Operator: ">=", Value: v})
	}
	if v := salesforce.OptionalString("warranty_ends_on_or_before", inputs); v != "" {
		scope = append(scope, salesforce.Condition{Field: "UsageEndDate", Operator: "<=", Value: v})
	}
	// The three lookup boxes are checked for ID SHAPE before they go anywhere near
	// a WHERE clause. Salesforce answers an ID field compared against a name with
	// a raw parser dump — the SOQL fragment, a caret, a row/column position and
	// INVALID_QUERY_FILTER_OPERATOR, a code that reads as though the operator's
	// Filter Comparison is at fault (verified live with account_id="Edge
	// Communications"). An upstream step that resolved to a company NAME rather
	// than an ID is the ordinary way to get here, and naming the box is the whole
	// difference between a fixable message and a support call.
	if v := salesforce.OptionalString("account_id", inputs); v != "" {
		if err := salesforce.ValidateRecordID(v); err != nil {
			return nil, fmt.Errorf("Customer (Account) — %w. Pick the account from the list, or pass the ID from an earlier Salesforce step rather than the company name", err)
		}
		filters = append(filters, salesforce.Condition{Field: "AccountId", Operator: "=", Value: v})
	}
	if v := salesforce.OptionalString("contact_id", inputs); v != "" {
		if err := salesforce.ValidateRecordID(v); err != nil {
			return nil, fmt.Errorf("Customer (Contact) — %w. Pick the contact from the list, or pass the ID from an earlier Salesforce step rather than a name or an email address", err)
		}
		filters = append(filters, salesforce.Condition{Field: "ContactId", Operator: "=", Value: v})
	}
	if v := salesforce.OptionalString("product_id", inputs); v != "" {
		if err := salesforce.ValidateRecordID(v); err != nil {
			return nil, fmt.Errorf("Product — %w. Pick the product from the list, or pass the ID from an earlier Salesforce step rather than the product name or code", err)
		}
		filters = append(filters, salesforce.Condition{Field: "Product2Id", Operator: "=", Value: v})
	}
	if v := salesforce.OptionalString("asset_status", inputs); v != "" {
		filters = append(filters, salesforce.Condition{Field: "Status", Operator: "=", Value: v})
	}
	if v := salesforce.OptionalString("serial_number", inputs); v != "" {
		filters = append(filters, salesforce.Condition{Field: "SerialNumber", Operator: "=", Value: v})
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

	// The typed builder, not the untyped one: Asset mixes text, currency, number,
	// tick-box and date fields, and whether Salesforce wants the value quoted
	// depends on which — Price > 5000 is valid where Price > '5000' is
	// INVALID_FIELD, IsCompetitorProduct = false must stay bare, and
	// UsageEndDate <= NEXT_N_DAYS:30 has to stay an unquoted keyword. One cached
	// describe resolves all of it, and a denied describe degrades rather than
	// failing the run.
	//
	// applyLimit is off for Return All: a LIMIT clause would cut the result short
	// at exactly the point the operator asked for everything.
	soql, err := salesforce.BuildScopedQueryTyped(
		instanceURL,
		token,
		"Asset",
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
			return salesforce.ErrorResult(fmt.Sprintf("assets are not available in your Salesforce org — an administrator can switch the Assets tab and object permissions on, and some Salesforce editions do not include them at all (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d asset(s) across %d page(s); stopped at the %d-page safety cap — narrow the filters to see the rest", len(records), pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d asset(s) across %d page(s)", len(records), pages)
	default:
		// Deliberately not "x of y": Salesforce reports totalSize as the PAGE size
		// once a LIMIT is applied and sets done:true with no cursor, so a capped
		// list is otherwise indistinguishable from a complete one.
		out["tool_result"] = fmt.Sprintf("Found %d asset(s)%s", len(records), salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}
