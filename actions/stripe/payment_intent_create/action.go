package payment_intent_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Payment Intent: Create"
	Description  = "Create a Stripe PaymentIntent to collect a payment. Returns the intent ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "amount", Type: core.ConnectionTypeMoney, Label: "Amount", Placeholder: "e.g. 12.34", Required: true},
	{Name: "currency", Type: core.ConnectionTypeString, Label: "Currency", Placeholder: "gbp", Required: true},
	{Name: "customer", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "cus_…"},
	{Name: "payment_method", Type: core.ConnectionTypeString, Label: "Payment Method", Placeholder: "pm_…"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "confirm", Type: core.ConnectionTypeBoolean, Label: "Confirm Immediately"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata", Placeholder: "Arbitrary key → value pairs stored on the payment intent"},
	{Name: "idempotency_key", Type: core.ConnectionTypeString, Label: "Idempotency Key", Placeholder: "Optional — safely retry the same create"},
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

	currency, err := stripe_common.RequiredString("currency", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}
	amount, err := stripe_common.MoneyToMinorUnits("amount", currency, inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}
	if amount == nil {
		return stripe_common.ErrorResult("amount is required"), nil
	}

	params := &stripe.PaymentIntentParams{}
	params.Amount = stripe.Int64(*amount)
	params.Currency = stripe.String(currency)
	if v := stripe_common.OptionalString("customer", inputs); v != "" {
		params.Customer = stripe.String(v)
	}
	if v := stripe_common.OptionalString("payment_method", inputs); v != "" {
		params.PaymentMethod = stripe.String(v)
	}
	if v := stripe_common.OptionalString("description", inputs); v != "" {
		params.Description = stripe.String(v)
	}
	if v := stripe_common.OptionalBool("confirm", inputs); v != nil {
		params.Confirm = stripe.Bool(*v)
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}
	if key := stripe_common.IdempotencyKey(inputs); key != "" {
		params.SetIdempotencyKey(key)
	}

	pi, err := stripe_common.NewClient(apiKey).PaymentIntents.New(params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(pi, fmt.Sprintf("Created payment intent %s", pi.ID)), nil
}
