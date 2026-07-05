package bank_transaction_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Bank Transaction: Create"
	Description  = "Create a Xero spend or receive bank transaction. Returns the transaction ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "type", Type: core.ConnectionTypeString, Label: "Type", Placeholder: "RECEIVE", Required: true, Options: []core.ConnectionOption{
		{Name: "Receive money (RECEIVE)", Value: "RECEIVE"},
		{Name: "Spend money (SPEND)", Value: "SPEND"},
	}},
	{Name: "contact_id", Type: core.ConnectionTypeString, Label: "Contact ID", Placeholder: "The Xero ContactID", Required: true},
	{Name: "bank_account_id", Type: core.ConnectionTypeString, Label: "Bank Account ID", Placeholder: "The bank AccountID", Required: true},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Date", Placeholder: "2026-07-05"},
	{Name: "reference", Type: core.ConnectionTypeString, Label: "Reference"},
	{Name: "line_amount_types", Type: core.ConnectionTypeString, Label: "Line Amount Types", Placeholder: "Inclusive"},
	{Name: "line_items", Type: core.ConnectionTypeText, Label: "Line Items (JSON array)", Placeholder: `[{"Description":"Item","Quantity":1,"UnitAmount":20.00,"AccountCode":"400"}]`},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"IsReconciled":false}`},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	txnType, err := xero_common.RequiredString("type", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	contactID, err := xero_common.RequiredString("contact_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	accountID, err := xero_common.RequiredString("bank_account_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"Type":        txnType,
		"Contact":     map[string]interface{}{"ContactID": contactID},
		"BankAccount": map[string]interface{}{"AccountID": accountID},
	}
	xero_common.SetString(body, "Date", "date", inputs)
	xero_common.SetString(body, "Reference", "reference", inputs)
	xero_common.SetString(body, "LineAmountTypes", "line_amount_types", inputs)

	lines, err := xero_common.ParseJSONArray("line_items", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	if lines != nil {
		body["LineItems"] = lines
	}

	extra, err := xero_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}
	xero_common.MergeFields(body, extra)

	resp, err := xero_common.DoJSON(flow, "POST", "/BankTransactions", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "BankTransactions")
	id, _ := obj["BankTransactionID"].(string)
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Created Xero bank transaction %s", id)), nil
}
