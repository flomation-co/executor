package payment_method_attach

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Payment Method: Attach"
	Description  = "Attach a Stripe payment method to a customer."
	Website      = "https://www.flomation.co"
	Icon         = "stripe"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "payment_method_id", Type: core.ConnectionTypeString, Label: "Payment Method ID", Placeholder: "pm_…", Required: true},
	{Name: "customer", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "cus_…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Payment Method ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Payment Method"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	id, err := stripe_common.RequiredString("payment_method_id", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}
	customer, err := stripe_common.RequiredString("customer", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}

	params := &stripe.PaymentMethodAttachParams{Customer: stripe.String(customer)}

	pm, err := stripe_common.NewClient(apiKey).PaymentMethods.Attach(id, params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(pm, fmt.Sprintf("Attached payment method %s to %s", pm.ID, customer)), nil
}
