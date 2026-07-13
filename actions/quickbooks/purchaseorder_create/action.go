package purchaseorder_create

import (
	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Purchase Order: Create"
	Description  = "Create a QuickBooks Online purchase order for a vendor with line items. Returns the purchase order ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "vendor_id", Type: core.ConnectionTypeString, Label: "Vendor ID", Placeholder: "56", Required: true},
	{Name: "ap_account_id", Type: core.ConnectionTypeString, Label: "AP Account ID", Placeholder: "33", Required: true},
	{Name: "line_items", Type: core.ConnectionTypeText, Label: "Line Items (JSON array)", Placeholder: `[{"Amount":100.0,"DetailType":"ItemBasedExpenseLineDetail","ItemBasedExpenseLineDetail":{"ItemRef":{"value":"1"}}}]`, Required: true},
	{Name: "txn_date", Type: core.ConnectionTypeString, Label: "Transaction Date", Placeholder: "2026-07-05"},
	{Name: "doc_number", Type: core.ConnectionTypeString, Label: "Document Number", Placeholder: "PO-1001"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"ShipAddr":{"Line1":"..."}}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	vendorID, err := quickbooks_common.RequiredString("vendor_id", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	apAccountID, err := quickbooks_common.RequiredString("ap_account_id", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	lines, err := quickbooks_common.ParseJSONArray("line_items", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	if len(lines) == 0 {
		return quickbooks_common.ErrorResult("line_items is required"), nil
	}

	body := map[string]interface{}{
		"VendorRef":    quickbooks_common.RefField(vendorID),
		"APAccountRef": quickbooks_common.RefField(apAccountID),
		"Line":         lines,
	}
	quickbooks_common.SetString(body, "TxnDate", "txn_date", inputs)
	quickbooks_common.SetString(body, "DocNumber", "doc_number", inputs)

	extra, err := quickbooks_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	quickbooks_common.MergeFields(body, extra)

	resp, err := quickbooks_common.Post(flow, auth, "purchaseorder", body)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "PurchaseOrder")
	id := quickbooks_common.IDOf(obj)
	return quickbooks_common.ObjectResult(id, obj, "Created QuickBooks purchase order "+id), nil
}
