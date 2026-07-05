package invoice_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Invoice: Update"
	Description  = "Update a QuickBooks Online invoice (sparse). Requires ID and sync token."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Invoice ID", Placeholder: "130", Required: true},
	{Name: "sync_token", Type: core.ConnectionTypeString, Label: "Sync Token", Placeholder: "0", Required: true},
	{Name: "txn_date", Type: core.ConnectionTypeString, Label: "Transaction Date"},
	{Name: "due_date", Type: core.ConnectionTypeString, Label: "Due Date"},
	{Name: "doc_number", Type: core.ConnectionTypeString, Label: "Document Number"},
	{Name: "line_items", Type: core.ConnectionTypeText, Label: "Line Items (JSON array)", Placeholder: `[{"Amount":100.0,"DetailType":"SalesItemLineDetail","SalesItemLineDetail":{"ItemRef":{"value":"1"}}}]`},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"CustomerMemo":{"value":"Updated"}}`},
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
		"sparse":    true,
	}
	quickbooks_common.SetString(body, "TxnDate", "txn_date", inputs)
	quickbooks_common.SetString(body, "DueDate", "due_date", inputs)
	quickbooks_common.SetString(body, "DocNumber", "doc_number", inputs)

	lines, err := quickbooks_common.ParseJSONArray("line_items", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	if len(lines) > 0 {
		body["Line"] = lines
	}

	extra, err := quickbooks_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	quickbooks_common.MergeFields(body, extra)

	resp, err := quickbooks_common.Post(flow, auth, "invoice", body)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "Invoice")
	return quickbooks_common.ObjectResult(quickbooks_common.IDOf(obj), obj, fmt.Sprintf("Updated QuickBooks invoice %q", id)), nil
}
