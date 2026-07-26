package crm_salesforce_product_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	salesforce "flomation.app/automate/executor/actions/crm/salesforce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Salesforce: Get Product"
	Description  = "Look up one product in your Salesforce catalogue by its record ID and return its details. Leave Fields blank to get everything the connected Salesforce user is allowed to see."
	Website      = "https://www.flomation.co"
	Icon         = "salesforce+magnifying-glass"
	Date         = "26/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Salesforce Connection", Placeholder: "Connect Salesforce, or paste an access token", Required: true},
	{Name: "instance_url", Type: core.ConnectionTypeString, Label: "Salesforce Instance URL", Placeholder: "Leave blank when you have connected Salesforce - only needed if you pasted a token yourself", FromCredentialMeta: "instance_url"},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product", Placeholder: "01t5f000004AbCdAAK - the record ID of the product to fetch", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Name,ProductCode,Family,IsActive (comma-separated; blank returns every field)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Product"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	instanceURL, token, err := salesforce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	productID := salesforce.OptionalString("product_id", inputs)
	if productID == "" {
		return nil, fmt.Errorf("product_id is required — the record ID of the product to fetch, e.g. 01t5f000004AbCdAAK")
	}
	// Catch a mistyped ID here so the operator gets "that is not a Salesforce
	// record ID" instead of a 404 that reads as though the product was deleted.
	if err := salesforce.ValidateRecordID(productID); err != nil {
		return nil, err
	}

	// A misspelled field name never reaches Salesforce, so it is a configuration
	// mistake and takes the hard error return. Left to GetRecord it would surface
	// on the soft error port, where it reads as a provider failure.
	fields := salesforce.OptionalString("fields", inputs)
	for _, f := range salesforce.SplitList(fields) {
		if _, err := salesforce.ValidateSOQLFieldName(f); err != nil {
			return nil, fmt.Errorf("Fields — %w", err)
		}
	}

	record, err := salesforce.GetRecord(instanceURL, token, "Product2", productID, fields)
	if err != nil {
		return salesforce.ErrorResult(err.Error()), nil
	}

	// The product record carries no price — price lives on PricebookEntry, one
	// per price book — so the summary names the product rather than implying a
	// figure the response does not contain.
	summary := fmt.Sprintf("Fetched product %s", productID)
	if name, ok := record["Name"].(string); ok && name != "" {
		summary = fmt.Sprintf("Fetched product %q (%s)", name, productID)
	}
	return salesforce.RecordResult(productID, record, summary), nil
}
