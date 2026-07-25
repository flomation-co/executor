package crm_salesforce_opportunity_line_item_update

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Update Opportunity Product"
	Description  = "Change the quantity, price or discount on a product line, and Salesforce recalculates the deal's value. Anything you leave blank is left exactly as it was."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+file-pen"
	Date         = "25/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "https://mycompany.my.salesforce.com", Required: true},
	{Name: "line_item_id", Type: core.ConnectionTypeString, Label: "Product Line ID", Placeholder: "00k5f00000AbCdEAAV - from Get Opportunity Products", Required: true},
	{Name: "quantity", Type: core.ConnectionTypeString, Label: "Quantity", Placeholder: "3 - how many of this product"},
	{Name: "unit_price", Type: core.ConnectionTypeString, Label: "Price Each", Placeholder: "499.00 - set this or Line Total, never both"},
	{Name: "total_price", Type: core.ConnectionTypeString, Label: "Line Total", Placeholder: "1497.00 - set this or Price Each, never both"},
	{Name: "discount", Type: core.ConnectionTypeString, Label: "Discount (%)", Placeholder: "10 — only if Discount is enabled on Opportunity Products in your org"},
	{Name: "service_date", Type: core.ConnectionTypeDateTime, Label: "Service Or Delivery Date", Placeholder: "2026-10-01 (the date only)"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Line Description", Placeholder: "What this line covers, shown on the deal"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "{\"My_Custom_Field__c\":\"value\"} - any other field on the product line"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product Line ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Applied Changes"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	id := salesforce.OptionalString("line_item_id", inputs)
	if err := salesforce.ValidateRecordID(id); err != nil {
		return nil, err
	}

	body := map[string]interface{}{}
	quantity, quantitySet, err := numericInput("quantity", "Quantity", inputs)
	if err != nil {
		return nil, err
	}
	if quantitySet {
		body["Quantity"] = quantity
	}

	// UnitPrice and TotalPrice are two views of the same number and Salesforce
	// rejects a payload that sets both — it derives whichever one you did not
	// send from the quantity.
	unitPrice, unitSet, err := numericInput("unit_price", "Price Each", inputs)
	if err != nil {
		return nil, err
	}
	totalPrice, totalSet, err := numericInput("total_price", "Line Total", inputs)
	if err != nil {
		return nil, err
	}
	if unitSet && totalSet {
		return nil, fmt.Errorf("set either Price Each or Line Total, not both — Salesforce works the other one out from the quantity")
	}
	if unitSet {
		body["UnitPrice"] = unitPrice
	}
	if totalSet {
		body["TotalPrice"] = totalPrice
	}

	discount, discountSet, err := numericInput("discount", "Discount", inputs)
	if err != nil {
		return nil, err
	}
	if discountSet {
		body["Discount"] = discount
	}
	// ServiceDate is a Date field — a full ISO timestamp is rejected outright.
	salesforce.SetDateIfPresent(body, inputs, "ServiceDate", "service_date")
	salesforce.SetIfPresent(body, inputs, "Description", "description")

	if err := salesforce.MergeAdditionalFields(body, inputs); err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("nothing to update — fill in at least one field to change on the product line")
	}

	if err := salesforce.UpdateRecord(instanceURL, token, "OpportunityLineItem", id, body); err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// Salesforce answers an update with 204 No Content, so echo the applied
	// changes rather than an empty result — the ID is what a follow-on node
	// needs and the field list is what the operator wants to see.
	changed := salesforce.SortedKeys(body)
	record := make(map[string]interface{}, len(body)+1)
	for k, v := range body {
		record[k] = v
	}
	record["Id"] = id

	return salesforce.RecordResult(id, record, fmt.Sprintf("Updated product line %s — changed %s", id, strings.Join(changed, ", "))), nil
}

// numericInput reads a decimal input, treating an unparseable value as the
// configuration mistake it is rather than silently dropping the field.
//
// OptionalFloat cannot tell "blank" from "£499" — both come back as unset, so a
// mistyped price would leave the line untouched while the run reported success.
func numericInput(name, label string, inputs []*core.Connection) (float64, bool, error) {
	raw := salesforce.OptionalString(name, inputs)
	if raw == "" {
		return 0, false, nil
	}
	v, ok := salesforce.OptionalFloat(name, inputs)
	if !ok {
		return 0, false, fmt.Errorf("%s must be a plain number such as 499.00 — got %q. Leave out currency symbols, thousands separators and spaces", label, raw)
	}
	return v, true, nil
}
