package account_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Account: Get"
	Description  = "Fetch a single Xero account by its ID. Returns the account object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+eye"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "account_id", Type: core.ConnectionTypeString, Label: "Account ID", Placeholder: "00000000-0000-0000-0000-000000000000", Required: true},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	id, err := xero_common.RequiredString("account_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	resp, err := xero_common.DoJSON(flow, "GET", "/Accounts/"+id, token, tenant, nil)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "Accounts")
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Fetched Xero account %s", id)), nil
}
