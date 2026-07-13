package account_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Account: Create"
	Description  = "Create a Xero chart-of-accounts account. Returns the account ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "code", Type: core.ConnectionTypeString, Label: "Code", Placeholder: "200"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Sales"},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "REVENUE", Required: true},
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

	body := map[string]interface{}{}
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
	id, _ := obj["AccountID"].(string)
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Created Xero account %q", xero_common.OptionalString("name", inputs))), nil
}
