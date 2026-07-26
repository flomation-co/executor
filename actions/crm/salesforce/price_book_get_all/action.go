package crm_salesforce_price_book_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Price Books"
	Description  = "List your Salesforce price books and see which one is the standard list price. Use it to find the price book ID a deal or a price needs, without hunting through Setup."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+layer-group"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// defaultFields is the projection used when the operator picks no fields.
//
// IsStandard leads alongside the name because it is the question this action
// exists to answer: every product must be priced on the standard price book
// before it can be priced anywhere else, so "which one is the standard?" is the
// first thing anyone needs. salesforce.DefaultFields has no Pricebook2 entry and
// would fall back to "Id,Name,LastModifiedDate", losing both flags.
//
// Every name here came from the live Pricebook2 describe — it is a short object:
// Name, Description, IsActive and IsStandard are the whole of it.
const defaultFields = "Id,Name,Description,IsActive,IsStandard,LastModifiedDate"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Name,IsActive,IsStandard - leave blank for the usual price book fields"},
	{Name: "standard_only", Type: core.ConnectionTypeBoolean, Label: "Only The Standard Price Book"},
	{Name: "active_only", Type: core.ConnectionTypeBoolean, Label: "Only Price Books In Use"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "Name - the field to filter on"},
	{
		Name:        "filter_operator",
		Type:        core.ConnectionTypeString,
		Label:       "Comparison",
		Placeholder: "How to compare the field to the value (defaults to equals)",
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "="},
			{Name: "Does Not Equal", Value: "!="},
			{Name: "Contains (LIKE)", Value: "LIKE"},
			{Name: "Does Not Contain (NOT LIKE)", Value: "NOT LIKE"},
			{Name: "Is One Of (IN)", Value: "IN"},
			{Name: "Is Not One Of (NOT IN)", Value: "NOT IN"},
			{Name: "Less Than", Value: "<"},
			{Name: "Less Than Or Equal To", Value: "<="},
			{Name: "Greater Than", Value: ">"},
			{Name: "Greater Than Or Equal To", Value: ">="},
		},
	},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "Trade with Contains - or Retail,Trade for Is One Of"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: "[{\"field\":\"IsActive\",\"operator\":\"=\",\"value\":\"true\"}] - extra filters"},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "Name ASC - or LastModifiedDate DESC"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted Price Books (Recycle Bin)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Price Books"},
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

	// SCOPE — always ANDed, never subject to the Match ANY toggle. "Only The
	// Standard Price Book" is an unconditional promise; ORing it away would return
	// every price book in the org, which is the opposite of what the box says.
	//
	// The two boxes are separate, and NEITHER is on by default, because they are
	// not the same question and can genuinely disagree. Verified live in a stock
	// Developer Edition org: the real standard price book (IsStandard=true) is
	// IsActive=FALSE, while a second, ordinary price book is literally NAMED
	// "Standard" with IsStandard=false and IsActive=true. So ticking "Only Price
	// Books In Use" hides the standard price book, and searching by the name
	// "Standard" finds the wrong record — which is exactly why this action reports
	// IsStandard rather than asking anyone to judge by the name.
	var scope []salesforce.Condition
	if salesforce.OptionalBool("standard_only", inputs) {
		scope = append(scope, salesforce.Condition{Field: "IsStandard", Operator: "=", Value: "true"})
	}
	if salesforce.OptionalBool("active_only", inputs) {
		scope = append(scope, salesforce.Condition{Field: "IsActive", Operator: "=", Value: "true"})
	}

	// FILTERS — the operator's own. The simple trio covers nearly everything and
	// needs no JSON; More Filters is the escape hatch. Both feed the same
	// validated WHERE builder.
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

	// The typed builder resolves each filter value against the field's real
	// Salesforce type via one cached describe, so "IsActive = true" renders as a
	// bare boolean — quoted, it is a hard INVALID_FIELD.
	soql, err := salesforce.BuildScopedQueryTyped(
		instanceURL,
		token,
		"Pricebook2",
		fields,
		scope,
		filters,
		salesforce.OptionalBool("match_any_filter", inputs),
		salesforce.OptionalString("order_by", inputs),
		clamped,
		!returnAll,
	)
	if err != nil {
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
		out["tool_result"] = fmt.Sprintf("Fetched %d %s across %d page(s); stopped at the %d-page safety cap", len(records), noun, pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d %s across %d page(s)", len(records), noun, pages)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d %s%s", len(records), noun, salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}

// plural picks the right noun for a count, so the summary an operator reads in
// the execution view is a sentence rather than "1 price books".
func plural(n int) string {
	if n == 1 {
		return "price book"
	}
	return "price books"
}
