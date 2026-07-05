package payment_intent_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Payment Intent: Update"
	Description  = "Update fields on an existing Stripe PaymentIntent."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "payment_intent_id", Type: core.ConnectionTypeString, Label: "Payment Intent ID", Placeholder: "pi_…", Required: true},
	{Name: "amount", Type: core.ConnectionTypeMoney, Label: "Amount", Placeholder: "e.g. 12.34"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Payment Intent ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Payment Intent"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	id, err := stripe_common.RequiredString("payment_intent_id", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}

	params := &stripe.PaymentIntentParams{}
	if v, err := stripe_common.MoneyToMinorUnits("amount", "", inputs); err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	} else if v != nil {
		params.Amount = stripe.Int64(*v)
	}
	if v := stripe_common.OptionalString("description", inputs); v != "" {
		params.Description = stripe.String(v)
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}

	pi, err := stripe_common.NewClient(apiKey).PaymentIntents.Update(id, params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(pi, fmt.Sprintf("Updated payment intent %s", pi.ID)), nil
}
