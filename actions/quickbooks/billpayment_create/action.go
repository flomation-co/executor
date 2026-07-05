package billpayment_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Bill Payment: Create"
	Description  = "Record a QuickBooks Online bill payment to a vendor. Returns the bill payment ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "vendor_id", Type: core.ConnectionTypeString, Label: "Vendor ID", Placeholder: "56", Required: true},
	{Name: "total_amount", Type: core.ConnectionTypeMoney, Label: "Total Amount", Placeholder: "100.00", Required: true},
	{Name: "pay_type", Type: core.ConnectionTypeString, Label: "Pay Type", Placeholder: "Check", Options: []core.ConnectionOption{{Name: "Check", Value: "Check"}, {Name: "Credit Card", Value: "CreditCard"}}},
	{Name: "line_items", Type: core.ConnectionTypeText, Label: "Line Items (JSON array)", Placeholder: `[{"Amount":100.00,"LinkedTxn":[{"TxnId":"145","TxnType":"Bill"}]}]`, Required: true},
	{Name: "txn_date", Type: core.ConnectionTypeString, Label: "Transaction Date", Placeholder: "2026-07-05"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Advanced Fields (JSON)", Placeholder: `{"CheckPayment":{"BankAccountRef":{"value":"36"}}}`},
}

var Outputs = quickbooks_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	vendorID, err := quickbooks_common.RequiredString("vendor_id", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	lines, err := quickbooks_common.ParseJSONArray("line_items", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	if len(lines) == 0 {
		return quickbooks_common.ErrorResult("line_items is required"), nil
	}

	body := map[string]interface{}{
		"VendorRef": quickbooks_common.RefField(vendorID),
		"Line":      lines,
	}
	quickbooks_common.SetNumber(body, "TotalAmt", "total_amount", inputs)
	quickbooks_common.SetString(body, "PayType", "pay_type", inputs)
	quickbooks_common.SetString(body, "TxnDate", "txn_date", inputs)

	extra, err := quickbooks_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}
	quickbooks_common.MergeFields(body, extra)

	resp, err := quickbooks_common.Post(flow, auth, "billpayment", body)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "BillPayment")
	id := quickbooks_common.IDOf(obj)
	return quickbooks_common.ObjectResult(id, obj, fmt.Sprintf("Recorded QuickBooks bill payment %s", id)), nil
}
