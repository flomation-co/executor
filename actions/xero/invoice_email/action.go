package invoice_email

import (
	"fmt"

	core "flomation.app/automate/executor"
	xero_common "flomation.app/automate/executor/actions/xero"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Invoice: Email"
	Description  = "Email a Xero invoice to its contact using the organisation's default template."
	Website      = "https://www.flomation.co"
	Icon         = "xero+paper-plane"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Xero Connection", Placeholder: "${credentials.MyXero}", Required: true},
	{Name: "tenant", Type: core.ConnectionTypeString, Label: "Organisation", Placeholder: "${credentials.MyXero.tenant_id}", Required: true},
	{Name: "invoice_id", Type: core.ConnectionTypeString, Label: "Invoice ID", Placeholder: "00000000-0000-0000-0000-000000000000", Required: true},
}

var Outputs = xero_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, tenant, err := xero_common.GetAuth(inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	id, err := xero_common.RequiredString("invoice_id", inputs)
	if err != nil {
		return xero_common.ErrorResult(err.Error()), nil
	}

	// Xero returns 204 No Content (empty body) on a successful email. DoJSON
	// treats any 2xx as success and returns a nil map for the empty body.
	_, err = xero_common.DoJSON(flow, "POST", "/Invoices/"+id+"/Email", token, tenant, nil)
	if err != nil {
		return xero_common.MapError(err), nil
	}

	return xero_common.ObjectResult(id, nil, fmt.Sprintf("Emailed invoice %s", id)), nil
}
