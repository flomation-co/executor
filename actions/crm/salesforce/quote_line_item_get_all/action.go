package crm_salesforce_quote_line_item_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Quote Products"
	Description  = "List the product lines on a quote with their quantity, price each, discount and line total - what you need to email the quote, build a PDF or turn it into an order."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+box"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

// defaultFields is the zero-configuration projection.
//
// It deliberately does NOT include Name or ProductCode. QuoteLineItem has neither
// field — verified against the live org's describe, and "SELECT Id, Name FROM
// QuoteLineItem" is a hard INVALID_FIELD. That trap is why the product's name is
// read through the relationship (Product2.Name) instead, which does work.
// LineNumber is Salesforce's own printed line reference and is what an operator
// matches against a paper quote.
const defaultFields = "Id,LineNumber,QuoteId,Product2Id,Product2.Name,PricebookEntryId,Quantity,UnitPrice,ListPrice,Discount,Subtotal,TotalPrice,ServiceDate,Description,SortOrder"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank - taken from your connection", FromCredentialMeta: "instance_url"},
	{Name: "quote_id", Type: core.ConnectionTypeString, Label: "Quote ID", Placeholder: "0Q05f000000AbCdAAK - the quote whose products you want", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Product2.Name,Quantity,UnitPrice,TotalPrice - leave blank for the usual quote-line fields"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "SortOrder ASC - or TotalPrice DESC"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every product line)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Quote Lines"},
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

	quoteID := salesforce.OptionalString("quote_id", inputs)
	if err := salesforce.ValidateRecordID(quoteID); err != nil {
		return nil, err
	}

	fields := salesforce.OptionalString("fields", inputs)
	if fields == "" {
		fields = defaultFields
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// There is no REST "list the children of this record" call for quote lines, so
	// this is a SOQL query — built through BuildQuery so the quote ID is escaped
	// and quoted rather than pasted into a string.
	soql, err := salesforce.BuildQuery(
		"QuoteLineItem",
		fields,
		[]salesforce.Condition{{Field: "QuoteId", Operator: "=", Value: quoteID}},
		false,
		salesforce.OptionalString("order_by", inputs),
		salesforce.ClampLimit(limit, limitSet),
		!returnAll,
	)
	if err != nil {
		return nil, err
	}

	records, nextURL, totalSize, pages, err := salesforce.Query(instanceURL, token, soql, returnAll, false)
	if err != nil {
		if salesforce.ErrorHasCode(err, "INVALID_TYPE") {
			return salesforce.ErrorResult(fmt.Sprintf(
				"quotes are switched off in your Salesforce org — an administrator can turn them on under Setup ▸ Quotes ▸ Quote Settings (%s)", err.Error())), nil
		}
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	if returnAll && nextURL != "" && pages >= salesforce.MaxAllPages {
		out["tool_result"] = fmt.Sprintf("Fetched %d product line(s) on quote %s across %d page(s); stopped at the %d-page safety cap", len(records), quoteID, pages, salesforce.MaxAllPages)
	} else {
		out["tool_result"] = fmt.Sprintf("Found %d product line(s) on quote %s%s", len(records), quoteID, salesforce.TruncationHint(len(records), limit, returnAll))
	}
	return out, nil
}
