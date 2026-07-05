package payout_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Payout: Create"
	Description  = "Create a Stripe payout to your bank account or debit card."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "amount", Type: core.ConnectionTypeMoney, Label: "Amount", Placeholder: "e.g. 12.34", Required: true},
	{Name: "currency", Type: core.ConnectionTypeString, Label: "Currency", Value: "gbp", Placeholder: "Three-letter ISO currency code, e.g. gbp", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Arbitrary string shown to users"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata", Placeholder: "Arbitrary key → value pairs stored on the payout"},
	{Name: "idempotency_key", Type: core.ConnectionTypeString, Label: "Idempotency Key", Placeholder: "Optional — safely retry the same create"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Payout ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Payout"},
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
	if amount == nil || *amount <= 0 {
		return stripe_common.ErrorResult("amount is required"), nil
	}

	params := &stripe.PayoutParams{}
	params.Amount = amount
	params.Currency = stripe.String(currency)
	if v := stripe_common.OptionalString("description", inputs); v != "" {
		params.Description = stripe.String(v)
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}
	if key := stripe_common.IdempotencyKey(inputs); key != "" {
		params.SetIdempotencyKey(key)
	}

	payout, err := stripe_common.NewClient(apiKey).Payouts.New(params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(payout, fmt.Sprintf("Created payout %s", payout.ID)), nil
}
