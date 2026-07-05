package bank_transfer_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Bank Transfer: Create"
	Description  = "Transfer money between two Xero bank accounts. Returns the transfer ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+repeat"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "from_account_id", Type: core.ConnectionTypeString, Label: "From Account ID", Placeholder: "The source bank AccountID", Required: true},
	{Name: "to_account_id", Type: core.ConnectionTypeString, Label: "To Account ID", Placeholder: "The destination bank AccountID", Required: true},
	{Name: "amount", Type: core.ConnectionTypeMoney, Label: "Amount", Placeholder: "100.00", Required: true},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Date", Placeholder: "2026-07-05"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"Reference":"Internal transfer"}`},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	fromID, err := xero_common.RequiredString("from_account_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	toID, err := xero_common.RequiredString("to_account_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"FromBankAccount": map[string]interface{}{"AccountID": fromID},
		"ToBankAccount":   map[string]interface{}{"AccountID": toID},
	}
	xero_common.SetNumber(body, "Amount", "amount", inputs)
	xero_common.SetString(body, "Date", "date", inputs)

	extra, err := xero_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	xero_common.MergeFields(body, extra)

	resp, err := xero_common.DoJSON(flow, "POST", "/BankTransfers", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "BankTransfers")
	id, _ := obj["BankTransferID"].(string)
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Created Xero bank transfer %s", id)), nil
}
