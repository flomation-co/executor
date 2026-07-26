package crm_salesforce_quote_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Many Quotes"
	Description  = "List quotes from Salesforce, optionally filtered - everything expiring this month, every quote on one deal, everything still waiting to be accepted. Turn on Return All to fetch every match."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+list"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// defaultFields is the projection used when the operator picks no fields.
//
// The quote number, the status, the money and the expiry are what anyone chasing
// quotes actually wants, so they lead. Subtotal, TotalPrice, GrandTotal and
// LineItemCount are computed by Salesforce from the quote's product lines and are
// read-only, which is exactly why they belong in a list projection: they are the
// figures you cannot get any other way without reading every line item.
//
// The customer takes TWO fields, not one. Quote.AccountId is read-only and comes
// from the parent opportunity, so it is null on every standalone quote — and a
// standalone quote is exactly what Create Quote's own "Company (Quote Account)"
// input makes, because it writes QuoteAccountId instead (verified live: a quote
// created that way reports AccountId null while QuoteAccountId holds the
// account). With only AccountId in the projection, "list quotes expiring this
// month, email the customer" came back with the customer column empty on every
// row and no error anywhere. Both are carried, each with its name hop, so the
// list names the customer whichever way the quote was raised.
const defaultFields = "Id,Name,QuoteNumber,Status,ExpirationDate,OpportunityId,AccountId,Account.Name,QuoteAccountId,QuoteAccount.Name,ContactId,Pricebook2Id,Subtotal,TotalPrice,GrandTotal,Discount,LineItemCount,IsSyncing,Email,Phone,LastModifiedDate"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "QuoteNumber,Status,GrandTotal,ExpirationDate - leave blank for the usual quote fields"},
	{Name: "filter_field", Type: core.ConnectionTypeString, Label: "Filter Field", Placeholder: "Status - the field to filter on"},
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
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "Presented - or a date word like THIS_MONTH, or Presented,Accepted for Is One Of"},
	{Name: "filter_conditions", Type: core.ConnectionTypeObject, Label: "More Filters", Placeholder: "[{\"field\":\"ExpirationDate\",\"operator\":\"<=\",\"value\":\"NEXT_N_DAYS:30\"},{\"field\":\"GrandTotal\",\"operator\":\">\",\"value\":\"10000\"}] - extra filters"},
	{Name: "match_any_filter", Type: core.ConnectionTypeBoolean, Label: "Match ANY filter instead of all"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "ExpirationDate ASC - or GrandTotal DESC"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},
	{Name: "include_deleted", Type: core.ConnectionTypeBoolean, Label: "Include Deleted Quotes (Recycle Bin)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Quotes"},
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

	// The simple field/comparison/value trio covers the overwhelming majority of
	// filters and needs no JSON. filter_conditions is the escape hatch for
	// anything more involved, and both feed the same validated WHERE builder —
	// nothing here concatenates a query by hand.
	var conditions []salesforce.Condition
	if field := salesforce.OptionalString("filter_field", inputs); field != "" {
		conditions = append(conditions, salesforce.Condition{
			Field:    field,
			Operator: salesforce.OptionalString("filter_operator", inputs),
			Value:    salesforce.OptionalString("filter_value", inputs),
		})
	}
	extra, err := salesforce.ParseConditions("filter_conditions", inputs)
	if err != nil {
		return nil, err
	}
	conditions = append(conditions, extra...)

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)
	clamped := salesforce.ClampLimit(limit, limitSet)

	// BuildQueryTyped, not BuildQuery: it resolves each filter value against the
	// field's real Salesforce type via one cached describe. On a quote that is
	// what makes "GrandTotal > 10000" render bare (currency), "ExpirationDate <=
	// NEXT_N_DAYS:30" render as a bare date keyword and "Status = 'Presented'"
	// render quoted. Under a value-only heuristic the first two are hard
	// INVALID_FIELD errors, and they are the two filters this action exists for.
	soql, err := salesforce.BuildQueryTyped(
		instanceURL,
		token,
		"Quote",
		fields,
		conditions,
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
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"quotes are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Quotes ▸ Quote Settings (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	noun := plural(len(records))
	switch {
	case returnAll && nextURL != "" && pages >= salesforce.MaxAllPages:
		out["tool_result"] = fmt.Sprintf("Fetched %d %s across %d page(s); stopped at the %d-page safety cap — narrow the filter or resume from the returned next page URL", len(records), noun, pages, salesforce.MaxAllPages)
	case returnAll:
		out["tool_result"] = fmt.Sprintf("Fetched all %d %s across %d page(s)", len(records), noun, pages)
	default:
		out["tool_result"] = fmt.Sprintf("Found %d %s%s", len(records), noun, salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}

// plural picks the right noun for a count, so the summary an operator reads in
// the execution view is a sentence rather than "1 quotes".
func plural(n int) string {
	if n == 1 {
		return "quote"
	}
	return "quotes"
}
