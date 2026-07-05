package payment_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Payment: Create"
	Description  = "Record a QuickBooks Online customer payment. Returns the payment ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "21", Required: true},
	{Name: "total_amount", Type: core.ConnectionTypeMoney, Label: "Total Amount", Placeholder: "100.00", Required: true},
	{Name: "line_items", Type: core.ConnectionTypeText, Label: "Line Items (JSON array)", Placeholder: `[{"Amount":100.00,"LinkedTxn":[{"TxnId":"96","TxnType":"Invoice"}]}]`},
	{Name: "txn_date", Type: core.ConnectionTypeString, Label: "Transaction Date", Placeholder: "2026-07-05"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"PrivateNote":"..."}`},
}

var Outputs = quickbooks_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	customerID, err := quickbooks_common.RequiredString("customer_id", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"CustomerRef": quickbooks_common.RefField(customerID),
	}
	quickbooks_common.SetNumber(body, "TotalAmt", "total_amount", inputs)
	quickbooks_common.SetString(body, "TxnDate", "txn_date", inputs)

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

	resp, err := quickbooks_common.Post(flow, auth, "payment", body)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "Payment")
	id := quickbooks_common.IDOf(obj)
	return quickbooks_common.ObjectResult(id, obj, fmt.Sprintf("Recorded QuickBooks payment %s", id)), nil
}
