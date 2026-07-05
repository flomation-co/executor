package invoice_send

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	quickbooks_common "flomation.app/automate/executor/actions/quickbooks"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Invoice: Send"
	Description  = "Email a QuickBooks Online invoice to the customer or a given address."
	Website      = "https://www.flomation.co"
	Icon         = "quickbooks+paper-plane"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "QuickBooks Connection", Placeholder: "${credentials.MyQBO}", Required: true},
	{Name: "company", Type: core.ConnectionTypeString, Label: "Company (Realm ID)", Placeholder: "${credentials.MyQBO.realm_id}", Required: true},
	{Name: "sandbox", Type: core.ConnectionTypeBoolean, Label: "Use Sandbox"},
	{Name: "invoice_id", Type: core.ConnectionTypeString, Label: "Invoice ID", Placeholder: "130", Required: true},
	{Name: "send_to", Type: core.ConnectionTypeString, Label: "Send To (Email)", Placeholder: "ada@example.com"},
}

var Outputs = quickbooks_common.StandardOutputs

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := quickbooks_common.GetAuth(inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	id, err := quickbooks_common.RequiredString("invoice_id", inputs)
	if err != nil {
		return quickbooks_common.ErrorResult(err.Error()), nil
	}

	entity := "invoice/" + id + "/send"
	if sendTo := quickbooks_common.OptionalString("send_to", inputs); sendTo != "" {
		entity += "?sendTo=" + url.QueryEscape(sendTo)
	}

	resp, err := quickbooks_common.Post(flow, auth, entity, nil)
	if err != nil {
		return quickbooks_common.MapError(err), nil
	}

	obj := quickbooks_common.Entity(resp, "Invoice")
	return quickbooks_common.ObjectResult(quickbooks_common.IDOf(obj), obj, fmt.Sprintf("Sent QuickBooks invoice %q", id)), nil
}
