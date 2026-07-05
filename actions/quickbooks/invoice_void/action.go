package invoice_void

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Invoice: Void"
	Description  = "Void a QuickBooks Online invoice. Requires ID and sync token."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+ban"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Invoice ID", Placeholder: "130", Required: true},
	{Name: "sync_token", Type: core.ConnectionTypeString, Label: "Sync Token", Placeholder: "0", Required: true},
}

var Outputs = quickbooks_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	id, err := quickbooks_common.RequiredString("id", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	syncToken, err := quickbooks_common.RequiredString("sync_token", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"Id":        id,
		"SyncToken": syncToken,
	}

	resp, err := quickbooks_common.Post(flow, auth, "invoice?operation=void", body)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "Invoice")
	return quickbooks_common.ObjectResult(quickbooks_common.IDOf(obj), obj, fmt.Sprintf("Voided QuickBooks invoice %q", id)), nil
}
