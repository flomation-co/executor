package crm_salesforce_price_book_entry_get_all

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Price Book Entries"
	Description  = "Find what a product costs, and get the price book entry ID you need to put it on a deal. Pick a price book, a product, or both - this is the lookup Salesforce makes you write SOQL for."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+dollar-sign"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// defaultFields is the projection used when the operator picks no fields.
//
// Name and ProductCode are read-only mirrors of the product's own name and code
// (live describe: createable=false on both), which is what makes an unaided list
// of price book entries readable at all — without them every row is three record
// IDs and a number.
//
// The two relationship hops are the point of the default, though: an operator
// reading this list wants to see "GenWatt Diesel 200kW, Standard, £25,000", not
// 01u.../01t.../01s... Traversal verified live against the org, including
// Pricebook2.IsStandard, which is how you tell the list price from a negotiated
// one. salesforce.DefaultFields has no PricebookEntry entry and its fallback
// ("Id,Name,LastModifiedDate") drops the price itself.
const defaultFields = "Id,Name,ProductCode,UnitPrice,IsActive,UseStandardPrice,Product2Id,Product2.Name,Product2.Family,Pricebook2Id,Pricebook2.Name,Pricebook2.IsStandard"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "pricebook_id", Type: core.ConnectionTypeString, Label: "Price Book", Placeholder: "01s5f000004AbCdAAK - only prices from this price book (leave blank for all of them)"},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - only prices for this product (leave blank for all of them)"},
	{Name: "product_code", Type: core.ConnectionTypeString, Label: "Product Code", Placeholder: "GC1040 - use this instead of Product if all you have is the catalogue code"},
	{Name: "active_only", Type: core.ConnectionTypeBoolean, Label: "Only Prices That Can Be Used"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "UnitPrice,Product2.Name - leave blank for the price, the product name and the price book name"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "UnitPrice - the field to filter on"},
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
			{Name: "Contains (LIKE)", Value: "LIKE"},
			{Name: "Does Not Contain (NOT LIKE)", Value: "NOT LIKE"},
			{Name: "Is One Of (IN)", Value: "IN"},
			{Name: "Is Not One Of (NOT IN)", Value: "NOT IN"},
		},
	},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "25000 with Greater Than - plain numbers only, no currency symbols or commas"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: "[{\"field\":\"UnitPrice\",\"operator\":\">\",\"value\":\"1000\"},{\"field\":\"UseStandardPrice\",\"operator\":\"=\",\"value\":\"false\"}] - extra filters"},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "UnitPrice DESC - or Name ASC"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Price Book Entries"},
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

	// SCOPE — the boundary boxes, always ANDed. "Price Book = X AND Product = Y"
	// is the whole point of this action; letting the Match ANY toggle turn it into
	// "OR" would hand back every price for the product plus every price in the
	// book, which is precisely the confusion the action exists to remove.
	var scope []salesforce.Condition
	// described names the subjects the list was narrowed to ("product X, price
	// book Y"); activeNote is kept separate because "for product code X, in use"
	// reads as a fourth subject rather than a qualifier.
	var described []string
	activeNote := ""

	if pricebookID := salesforce.OptionalString("pricebook_id", inputs); pricebookID != "" {
		if err := salesforce.ValidateRecordID(pricebookID); err != nil {
			return nil, fmt.Errorf("Price Book — %w", err)
		}
		scope = append(scope, salesforce.Condition{Field: "Pricebook2Id", Operator: "=", Value: pricebookID})
		described = append(described, fmt.Sprintf("price book %s", pricebookID))
	}
	if productID := salesforce.OptionalString("product_id", inputs); productID != "" {
		if err := salesforce.ValidateRecordID(productID); err != nil {
			return nil, fmt.Errorf("Product — %w", err)
		}
		scope = append(scope, salesforce.Condition{Field: "Product2Id", Operator: "=", Value: productID})
		described = append(described, fmt.Sprintf("product %s", productID))
	}
	// ProductCode on PricebookEntry is a read-only mirror of the product's code
	// (live describe), but it IS filterable — which makes it the escape hatch for
	// a flow that came from a spreadsheet or a shop and has a catalogue code
	// rather than a Salesforce ID.
	if productCode := salesforce.OptionalString("product_code", inputs); productCode != "" {
		scope = append(scope, salesforce.Condition{Field: "ProductCode", Operator: "=", Value: productCode})
		described = append(described, fmt.Sprintf("product code %q", productCode))
	}
	if salesforce.OptionalBool("active_only", inputs) {
		// Salesforce defaults PricebookEntry.IsActive to FALSE (verified on the
		// live describe), so inactive entries are common in a catalogue that was
		// loaded through the API — and an inactive entry cannot be put on a deal.
		scope = append(scope, salesforce.Condition{Field: "IsActive", Operator: "=", Value: "true"})
		activeNote = ", counting only prices that can be used"
	}

	// FILTERS — the operator's own.
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

	// BuildScopedQueryTyped matters more here than on any other list in this
	// group: UnitPrice is a Currency field, so "UnitPrice > 1000" must render as a
	// BARE number. Quoted, Salesforce answers INVALID_FIELD outright — and
	// "everything over a thousand pounds" is the first filter anyone reaches for
	// on a price list.
	soql, err := salesforce.BuildScopedQueryTyped(
		instanceURL,
		token,
		"PricebookEntry",
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

	// includeDeleted is deliberately not offered: PricebookEntry rows in the
	// Recycle Bin are the debris of deleted products, and returning one would hand
	// the operator an entry ID that Salesforce refuses to put on a deal.
	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, false)
	if err != nil {
		// INVALID_TYPE here means Products is not switched on for the connected
		// user rather than a bad field name, and common.go already says so.
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	noun := plural(len(records))
	where := ""
	if len(described) > 0 {
		where = " for " + strings.Join(described, ", ")
	}
	where += activeNote
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d %s%s across %d page(s); stopped at the %d-page safety cap", len(records), noun, where, pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d %s%s across %d page(s)", len(records), noun, where, pages)
	case len(records) == 0 && len(scope) > 0:
		// An empty result with a scope set is the single most likely outcome an
		// operator will hit here, and "Found 0" tells them nothing about which of
		// the two dropdowns was wrong. A product genuinely absent from a price book
		// is normal — Salesforce does not price every product on every book — and
		// the fix is Add Product to Price Book, so say so.
		out["tool_result"] = fmt.Sprintf("Found no prices%s — a product only has a price in the price books it has been added to, so add it with Add Product to Price Book, or check the Price Book and Product you picked", where)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d %s%s%s", len(records), noun, where, salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}

// plural picks the right noun for a count, so the summary an operator reads in
// the execution view is a sentence rather than "1 prices".
func plural(n int) string {
	if n == 1 {
		return "price"
	}
	return "prices"
}
