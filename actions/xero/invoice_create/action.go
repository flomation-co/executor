package invoice_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Invoice: Create"
	Description  = "Create a Xero invoice (sales ACCREC or bill ACCPAY) with line items. Returns the invoice ID."
	Website      = "https://www.flomation.co"
	Icon         = "xero+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "00000000-0000-0000-0000-000000000000", Required: true},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "ACCREC", Options: []core.ConnectionOption{
		{Name: "Sales Invoice (ACCREC)", Value: "ACCREC"},
		{Name: "Bill (ACCPAY)", Value: "ACCPAY"},
	}},
	{Name: "line_items", Type: core.ConnectionTypeText, Label: "Line Items (JSON)", Placeholder: `[{"Description":"Consulting","Quantity":1,"UnitAmount":100.00,"AccountCode":"200"}]`, Required: true},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Date", Placeholder: "2026-07-05"},
	{Name: "due_date", Type: core.ConnectionTypeString, Label: "Due Date", Placeholder: "2026-08-05"},
	{Name: "reference", Type: core.ConnectionTypeString, Label: "Reference", Placeholder: "INV-001"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "DRAFT", Options: []core.ConnectionOption{
		{Name: "Draft", Value: "DRAFT"},
		{Name: "Submitted", Value: "SUBMITTED"},
		{Name: "Authorised", Value: "AUTHORISED"},
	}},
	{Name: "line_amount_types", Type: core.ConnectionTypeString, Label: "Line Amount Types", Placeholder: "Exclusive", Options: []core.ConnectionOption{
		{Name: "Exclusive", Value: "Exclusive"},
		{Name: "Inclusive", Value: "Inclusive"},
		{Name: "No Tax", Value: "NoTax"},
	}},
	{Name: "currency_code", Type: core.ConnectionTypeString, Label: "Currency Code", Placeholder: "GBP"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"BrandingThemeID":"..."}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	contactID, err := xero_common.RequiredString("contact_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	lineItems, err := xero_common.ParseJSONArray("line_items", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	if len(lineItems) == 0 {
		return xero_common.ErrorResult("line_items is required (a JSON array of line objects)"), nil
	}

	invType := xero_common.OptionalString("type", inputs)
	if invType == "" {
		invType = "ACCREC"
	}

	body := map[string]interface{}{
		"Type":      invType,
		"Contact":   map[string]interface{}{"ContactID": contactID},
		"LineItems": lineItems,
	}
	xero_common.SetString(body, "Date", "date", inputs)
	xero_common.SetString(body, "DueDate", "due_date", inputs)
	xero_common.SetString(body, "Reference", "reference", inputs)
	xero_common.SetString(body, "Status", "status", inputs)
	xero_common.SetString(body, "LineAmountTypes", "line_amount_types", inputs)
	xero_common.SetString(body, "CurrencyCode", "currency_code", inputs)

	extra, err := xero_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	xero_common.MergeFields(body, extra)

	resp, err := xero_common.DoJSON(flow, "POST", "/Invoices", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "Invoices")
	id, _ := obj["InvoiceID"].(string)
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Created Xero %s invoice", invType)), nil
}
