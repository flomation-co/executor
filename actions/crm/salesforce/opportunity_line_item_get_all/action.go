package crm_salesforce_opportunity_line_item_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Opportunity Products"
	Description  = "List the product lines on a deal with their quantity, price each and line total - what you need to build a quote, an invoice or an order from the deal."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+box"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

// defaultFields is the projection used when the operator picks no fields.
// Name is the product's name as it appears on the line, and the quantity/price
// trio is the whole reason anyone reads this list — so all four lead.
// defaultFields is the zero-configuration projection. It deliberately does NOT
// include Discount: that field is absent from OpportunityLineItem unless the org
// has enabled it, and a stock Developer Edition org does not — so the default
// path failed with INVALID_FIELD ("No such column 'Discount'") for an operator
// who did exactly what the placeholder told them and left Fields blank.
// ListPrice is present everywhere and is the more useful figure anyway.
const defaultFields = "Id,Name,OpportunityId,Product2Id,PricebookEntryId,ProductCode,Quantity,UnitPrice,ListPrice,TotalPrice,ServiceDate,Description"

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "opportunity_id", Type: core.ConnectionTypeString, Label: "Opportunity ID", Placeholder: "0065f00000AbCdEAAV - the deal whose products you want", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Name,Quantity,UnitPrice - leave blank for the usual product-line fields"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "TotalPrice DESC - or ServiceDate ASC"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (fetch every product line)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 (max 2000); ignored when Return All is on"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Product Lines"},
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

	opportunityID := salesforce.OptionalString("opportunity_id", inputs)
	if err := salesforce.ValidateRecordID(opportunityID); err != nil {
		return nil, err
	}

	fields := salesforce.OptionalString("fields", inputs)
	if fields == "" {
		fields = defaultFields
	}

	returnAll := salesforce.OptionalBool("return_all", inputs)
	limit, limitSet := salesforce.OptionalInt("limit", inputs)

	// There is no REST "list the children of this record" call for product
	// lines, so this is a SOQL query — built through BuildQuery so the deal ID
	// is escaped and quoted rather than pasted into a string.
	soql, err := salesforce.BuildQuery(
		"OpportunityLineItem",
		fields,
		[]salesforce.Condition{{Field: "OpportunityId", Operator: "=", Value: opportunityID}},
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
		return salesforce.ErrorResult(err.Error()), nil
	}

	out := salesforce.ListResult(records, nextURL, totalSize, "")
	if returnAll && nextURL != "" && pages >= salesforce.MaxAllPages {
		out["tool_result"] = fmt.Sprintf("Fetched %d product line(s) on opportunity %s across %d page(s); stopped at the %d-page safety cap", len(records), opportunityID, pages, salesforce.MaxAllPages)
	} else {
		out["tool_result"] = fmt.Sprintf("Found %d product line(s) on opportunity %s", len(records), opportunityID)
	}
	return out, nil
}
