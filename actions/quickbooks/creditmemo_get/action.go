package creditmemo_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Credit Memo: Get"
	Description  = "Fetch a QuickBooks Online credit memo by ID. Returns the credit memo object."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+eye"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "credit_memo_id", Type: core.ConnectionTypeString, Label: "Credit Memo ID", Placeholder: "201", Required: true},
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

	id, err := quickbooks_common.RequiredString("credit_memo_id", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	resp, err := quickbooks_common.GetByID(flow, auth, "creditmemo", id)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "CreditMemo")
	return quickbooks_common.ObjectResult(quickbooks_common.IDOf(obj), obj, fmt.Sprintf("Fetched QuickBooks credit memo %s", id)), nil
}
