package crm_salesforce_product_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Products"
	Description  = "List products from your Salesforce catalogue - everything you sell, one product family, or just the items that are still ready to sell. Turn on Return All to fetch every match."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// defaultFields is the projection used when the operator picks no fields.
//
// It is spelled out rather than left to salesforce.DefaultFields, which has no
// entry for Product2 and would fall back to "Id,Name,LastModifiedDate" — a list
// with no product code and no active flag, which is most of the reason anyone
// reads a catalogue.
//
// Every name here was taken from the live Product2 describe. Note what is NOT in
// it: there is no price field on Product2 at all. A product's price lives on
// PricebookEntry, one per price book, so "SELECT ... UnitPrice FROM Product2" is
// a hard INVALID_FIELD — use Get Many Price Book Entries for prices.
const defaultFields = "Id,Name,ProductCode,Family,IsActive,QuantityUnitOfMeasure,StockKeepingUnit,Description,LastModifiedDate"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Name,ProductCode,Family - leave blank for the usual catalogue fields"},
	{Name: "active_only", Type: core.ConnectionTypeBoolean, Label: "Only Products Ready To Sell"},
	{Name: "family", Type: core.ConnectionTypeString, Label: "Product Family", Placeholder: "Generators - must match a value in your org's Product Family list exactly, or nothing comes back"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "ProductCode - the field to filter on"},
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
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "GC10% with Contains - or GC1040,GC1060 for Is One Of, or a date word like LAST_N_DAYS:30"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: "[{\"field\":\"IsActive\",\"operator\":\"=\",\"value\":\"true\"},{\"field\":\"Name\",\"operator\":\"LIKE\",\"value\":\"GenWatt%\"}] - extra filters"},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "Name ASC - or LastModifiedDate DESC"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted Products (Recycle Bin)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Products"},
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

	// SCOPE — the action's own boundary boxes. These are always ANDed: "Only
	// Products Ready To Sell" is an unconditional promise, and letting the Match
	// ANY toggle OR it away would hand back the retired products the box exists
	// to hide. See scoped_query.go for the live-verified reasoning.
	var scope []salesforce.Condition
	if salesforce.OptionalBool("active_only", inputs) {
		scope = append(scope, salesforce.Condition{Field: "IsActive", Operator: "=", Value: "true"})
	}
	// Family is an unrestricted picklist whose list is empty in a stock org, so a
	// value that does not match the org's own list simply matches no rows. The
	// summary below therefore names the family, so an empty result reads as "none
	// in that family" rather than "no products".
	if family := salesforce.OptionalString("family", inputs); family != "" {
		scope = append(scope, salesforce.Condition{Field: "Family", Operator: "=", Value: family})
	}

	// FILTERS — the operator's own. The simple trio covers nearly everything and
	// needs no JSON; More Filters is the escape hatch. Both feed the same
	// validated WHERE builder, so nothing here concatenates a query by hand.
	var filters []salesforce.Condition
	if field := salesforce.OptionalString("filter_field", inputs); field != "" {
		filters = append(filters, salesforce.Condition{
			Field:    field,
			Operator: salesforce.OptionalString("filter_operator", inputs),
			Value:    salesforce.OptionalString("filter_value", inputs),
		})
	}
	extra, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	filters = append(filters, extra...)

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	clamped := salesforce.ClampLimit(limit, limitSet)

	// BuildScopedQueryTyped, not BuildQuery: the typed path resolves each filter
	// value against the field's real Salesforce type via one cached describe, so
	// "IsActive = true" renders as a bare boolean rather than the quoted string
	// Salesforce rejects outright with INVALID_FIELD.
	soql, err := salesforce.BuildScopedQueryTyped(
		instanceURL,
		token,
		"Product2",
		fields,
		scope,
		filters,
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
	where := ""
	if family := salesforce.OptionalString("family", inputs); family != "" {
		where = fmt.Sprintf(" in the %q family", family)
	}
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d %s%s across %d page(s); stopped at the %d-page safety cap — narrow the filter or resume from the returned next page URL", len(records), noun, where, pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d %s%s across %d page(s)", len(records), noun, where, pages)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d %s%s%s", len(records), noun, where, salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}

// plural picks the right noun for a count, so the summary an operator reads in
// the execution view is a sentence rather than "1 products".
func plural(n int) string {
	if n == 1 {
		return "product"
	}
	return "products"
}
