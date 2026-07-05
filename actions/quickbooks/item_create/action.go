package item_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Item: Create"
	Description  = "Create a QuickBooks Online item (product or service). Returns the item ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Consulting", Required: true},
	{Name: "item_type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "Service", Required: true, Options: []core.ConnectionOption{{Name: "Service", Value: "Service"}, {Name: "Inventory", Value: "Inventory"}, {Name: "Non-Inventory", Value: "NonInventory"}}},
	{Name: "income_account_id", Type: core.ConnectionTypeString, Label: "Income Account ID", Placeholder: "79", Required: true},
	{Name: "unit_price", Type: core.ConnectionTypeMoney, Label: "Unit Price", Placeholder: "150.00"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"Description":"...","Taxable":true}`},
}

var Outputs = quickbooks_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	if _, err := quickbooks_common.RequiredString("name", inputs); err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	if _, err := quickbooks_common.RequiredString("item_type", inputs); err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	incomeAccountID, err := quickbooks_common.RequiredString("income_account_id", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"IncomeAccountRef": quickbooks_common.RefField(incomeAccountID),
	}
	quickbooks_common.SetString(body, "Name", "name", inputs)
	quickbooks_common.SetString(body, "Type", "item_type", inputs)
	quickbooks_common.SetNumber(body, "UnitPrice", "unit_price", inputs)

	extra, err := quickbooks_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	quickbooks_common.MergeFields(body, extra)

	resp, err := quickbooks_common.Post(flow, auth, "item", body)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "Item")
	id := quickbooks_common.IDOf(obj)
	return quickbooks_common.ObjectResult(id, obj, fmt.Sprintf("Created QuickBooks item %q", quickbooks_common.OptionalString("name", inputs))), nil
}
