package account_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Account: Create"
	Description  = "Create a QuickBooks Online chart-of-accounts account. Returns the account ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Account Name", Placeholder: "Marketing Expenses", Required: true},
	{Name: "account_type", Type: core.ConnectionTypeString, Label: "Account Type", Placeholder: "Expense"},
	{Name: "account_sub_type", Type: core.ConnectionTypeString, Label: "Account Sub-Type", Placeholder: "AdvertisingPromotional"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"AcctNum":"6000","Description":"..."}`},
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

	body := map[string]interface{}{}
	quickbooks_common.SetString(body, "Name", "name", inputs)
	quickbooks_common.SetString(body, "AccountType", "account_type", inputs)
	quickbooks_common.SetString(body, "AccountSubType", "account_sub_type", inputs)

	extra, err := quickbooks_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	quickbooks_common.MergeFields(body, extra)

	resp, err := quickbooks_common.Post(flow, auth, "account", body)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "Account")
	id := quickbooks_common.IDOf(obj)
	return quickbooks_common.ObjectResult(id, obj, fmt.Sprintf("Created QuickBooks account %q", quickbooks_common.OptionalString("name", inputs))), nil
}
