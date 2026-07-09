package purchase_order_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Purchase Order: Update"
	Description  = "Update an existing Xero purchase order by its ID. Returns the updated object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "purchase_order_id", Type: core.ConnectionTypeString, Label: "Purchase Order ID", Placeholder: "00000000-0000-0000-0000-000000000000", Required: true},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Supplier Contact ID", Placeholder: "00000000-0000-0000-0000-000000000000"},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Date", Placeholder: "2026-07-05"},
	{Name: "delivery_date", Type: core.ConnectionTypeString, Label: "Delivery Date", Placeholder: "2026-07-12"},
	{Name: "reference", Type: core.ConnectionTypeString, Label: "Reference", Placeholder: "PO-1001"},
	{Name: "line_items", Type: core.ConnectionTypeText, Label: "Line Items (JSON array)", Placeholder: `[{"Description":"Widgets","Quantity":10,"UnitAmount":5.0,"AccountCode":"300"}]`},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"Status":"AUTHORISED"}`},
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

	id, err := xero_common.RequiredString("purchase_order_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"PurchaseOrderID": id}
	if contactID := xero_common.OptionalString("contact_id", inputs); contactID != "" {
		body["Contact"] = map[string]interface{}{"ContactID": contactID}
	}
	xero_common.SetString(body, "Date", "date", inputs)
	xero_common.SetString(body, "DeliveryDate", "delivery_date", inputs)
	xero_common.SetString(body, "Reference", "reference", inputs)

	lineItems, err := xero_common.ParseJSONArray("line_items", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	if lineItems != nil {
		body["LineItems"] = lineItems
	}

	extra, err := xero_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	xero_common.MergeFields(body, extra)

	resp, err := xero_common.DoJSON(flow, "POST", "/PurchaseOrders", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "PurchaseOrders")
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Updated Xero purchase order %s", id)), nil
}
