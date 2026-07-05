package payment_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Payment: Get"
	Description  = "Fetch a single Xero payment by ID. Returns the payment object."
	Website      = "https://www.flomation.co"
	Icon         = "xero+eye"
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

	resp, err := xero_common.DoJSON(flow, "GET", "/Payments/"+id, token, tenant, nil)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	obj := xero_common.FirstElement(resp, "Payments")
	gotID, _ := obj["PaymentID"].(string)
	return xero_common.ObjectResult(gotID, obj, fmt.Sprintf("Fetched Xero payment %s", id)), nil
}
