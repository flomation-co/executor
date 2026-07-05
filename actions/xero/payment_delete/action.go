package payment_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Payment: Delete"
	Description  = "Delete (reverse) a Xero payment by setting its status to DELETED."
	Website      = "https://www.flomation.co"
	Icon         = "xero+trash"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "payment_id", Type: core.ConnectionTypeString, Label: "Payment ID", Placeholder: "The Xero PaymentID", Required: true},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	id, err := xero_common.RequiredString("payment_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"PaymentID": id,
		"Status":    "DELETED",
	}

	resp, err := xero_common.DoJSON(flow, "POST", "/Payments", token, tenant, body)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "Payments")
	return xero_common.ObjectResult(id, obj, fmt.Sprintf("Deleted Xero payment %s", id)), nil
}
