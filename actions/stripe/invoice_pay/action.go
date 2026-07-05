package invoice_pay

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Invoice: Pay"
	Description  = "Attempt to collect payment on a Stripe invoice out of the normal schedule."
	Website      = "https://www.flomation.co"
	Icon         = "stripe"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "invoice_id", Type: core.ConnectionTypeString, Label: "Invoice ID", Placeholder: "in_…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Invoice ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Invoice"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	id, err := stripe_common.RequiredString("invoice_id", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}

	inv, err := stripe_common.NewClient(apiKey).Invoices.Pay(id, &stripe.InvoicePayParams{})
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(inv, fmt.Sprintf("Paid invoice %s", inv.ID)), nil
}
