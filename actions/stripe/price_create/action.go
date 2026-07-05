package price_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Price: Create"
	Description  = "Create a Stripe price for a product. Returns the price ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "product", Type: core.ConnectionTypeString, Label: "Product ID", Placeholder: "prod_…", Required: true},
	{Name: "unit_amount", Type: core.ConnectionTypeMoney, Label: "Unit Amount", Placeholder: "e.g. 12.34", Required: true},
	{Name: "currency", Type: core.ConnectionTypeString, Label: "Currency", Placeholder: "gbp", Required: true},
	{Name: "recurring_interval", Type: core.ConnectionTypeString, Label: "Recurring Interval", Options: []core.ConnectionOption{
		{Name: "One-off", Value: ""},
		{Name: "Daily", Value: "day"},
		{Name: "Weekly", Value: "week"},
		{Name: "Monthly", Value: "month"},
		{Name: "Yearly", Value: "year"},
	}},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata", Placeholder: "Arbitrary key → value pairs stored on the price"},
	{Name: "idempotency_key", Type: core.ConnectionTypeString, Label: "Idempotency Key", Placeholder: "Optional — safely retry the same create"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Price ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Price"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	product, err := stripe_common.RequiredString("product", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}
	currency, err := stripe_common.RequiredString("currency", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}
	amount, err := stripe_common.MoneyToMinorUnits("unit_amount", currency, inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}
	if amount == nil {
		return stripe_common.ErrorResult("unit_amount is required"), nil
	}

	params := &stripe.PriceParams{
		Product:    stripe.String(product),
		UnitAmount: amount,
		Currency:   stripe.String(currency),
	}
	if v := stripe_common.OptionalString("recurring_interval", inputs); v != "" {
		params.Recurring = &stripe.PriceRecurringParams{Interval: stripe.String(v)}
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}
	if key := stripe_common.IdempotencyKey(inputs); key != "" {
		params.SetIdempotencyKey(key)
	}

	price, err := stripe_common.NewClient(apiKey).Prices.New(params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(price, fmt.Sprintf("Created price %s", price.ID)), nil
}
