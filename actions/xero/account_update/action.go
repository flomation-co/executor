package account_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Account: Update"
	Description  = "Update an existing Xero account by its ID. Returns the updated account object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account ID", Placeholder: "00000000-0000-0000-0000-000000000000", Required: true},
	{Name: "code", Type: core.ConnectionTypeString, Label: "Code", Placeholder: "200"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Sales"},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "REVENUE"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"TaxType":"OUTPUT2"}`},
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

	id, err := xero_common.RequiredString("account_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"AccountID": id}
	xero_common.SetString(body, "Code", "code", inputs)
	xero_common.SetString(body, "Name", "name", inputs)
	xero_common.SetString(body, "Type", "type", inputs)
	xero_common.SetString(body, "Description", "description", inputs)

	extra, err := xero_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	xero_common.MergeFields(body, extra)

	resp, err := xero_common.DoJSON(flow, "POST", "/Accounts", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "Accounts")
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Updated Xero account %s", id)), nil
}
