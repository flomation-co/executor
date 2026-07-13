package invoice_void

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Invoice: Void"
	Description  = "Void a Xero invoice by ID (sets its status to VOIDED). Returns the voided invoice."
	Website      = "https://www.flomation.co"
	Icon         = "xero+ban"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "invoice_id", Type: core.ConnectionTypeString, Label: "Invoice ID", Placeholder: "00000000-0000-0000-0000-000000000000", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Object ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	id, err := xero_common.RequiredString("invoice_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"InvoiceID": id,
		"Status":    "VOIDED",
	}

	resp, err := xero_common.DoJSON(flow, "POST", "/Invoices", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "Invoices")
	iid, _ := obj["InvoiceID"].(string)
	return xero_common.ObjectResult(iid, obj, fmt.Sprintf("Voided Xero invoice %q", id)), nil
}
