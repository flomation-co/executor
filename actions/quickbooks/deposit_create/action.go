package deposit_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Deposit: Create"
	Description  = "Create a QuickBooks Online deposit into an account. Returns the deposit ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "deposit_to_account_id", Type: core.ConnectionTypeString, Label: "Deposit To Account ID", Placeholder: "35", Required: true},
	{Name: "line_items", Type: core.ConnectionTypeText, Label: "Lines (JSON array)", Placeholder: `[{"Amount":100.0,"DetailType":"DepositLineDetail","DepositLineDetail":{"AccountRef":{"value":"79"}}}]`, Required: true},
	{Name: "txn_date", Type: core.ConnectionTypeString, Label: "Transaction Date", Placeholder: "2026-07-05"},
	{Name: "private_note", Type: core.ConnectionTypeString, Label: "Private Note (Memo)"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"CurrencyRef":{"value":"GBP"}}`},
}

var Outputs = quickbooks_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	accountID, err := quickbooks_common.RequiredString("deposit_to_account_id", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	lines, err := quickbooks_common.ParseJSONArray("line_items", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	if len(lines) == 0 {
		return quickbooks_common.ErrorResult("line_items is required — provide at least one deposit line"), nil
	}

	body := map[string]interface{}{
		"DepositToAccountRef": quickbooks_common.RefField(accountID),
		"Line":                lines,
	}
	quickbooks_common.SetString(body, "TxnDate", "txn_date", inputs)
	quickbooks_common.SetString(body, "PrivateNote", "private_note", inputs)

	extra, err := quickbooks_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	quickbooks_common.MergeFields(body, extra)

	resp, err := quickbooks_common.Post(flow, auth, "deposit", body)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "Deposit")
	id := quickbooks_common.IDOf(obj)
	return quickbooks_common.ObjectResult(id, obj, fmt.Sprintf("Created QuickBooks deposit %s", id)), nil
}
