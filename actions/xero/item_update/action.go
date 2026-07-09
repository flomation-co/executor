package item_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Item: Update"
	Description  = "Update an existing Xero item by its ID. Returns the updated item object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "item_id", Type: core.ConnectionTypeString, Label: "Item ID", Placeholder: "00000000-0000-0000-0000-000000000000", Required: true},
	{Name: "code", Type: core.ConnectionTypeString, Label: "Code", Placeholder: "WIDGET-01"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Blue Widget"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Standard blue widget"},
	{Name: "is_sold", Type: core.ConnectionTypeBoolean, Label: "Is Sold"},
	{Name: "is_purchased", Type: core.ConnectionTypeBoolean, Label: "Is Purchased"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"SalesDetails":{"UnitPrice":10.0,"AccountCode":"200"}}`},
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

	id, err := xero_common.RequiredString("item_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"ItemID": id}
	xero_common.SetString(body, "Code", "code", inputs)
	xero_common.SetString(body, "Name", "name", inputs)
	xero_common.SetString(body, "Description", "description", inputs)
	if v := xero_common.OptionalBool("is_sold", inputs); v != nil {
		body["IsSold"] = *v
	}
	if v := xero_common.OptionalBool("is_purchased", inputs); v != nil {
		body["IsPurchased"] = *v
	}

	extra, err := xero_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	xero_common.MergeFields(body, extra)

	resp, err := xero_common.DoJSON(flow, "POST", "/Items", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "Items")
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Updated Xero item %s", id)), nil
}
