package coupon_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Coupon: Create"
	Description  = "Create a Stripe coupon (percent-off or amount-off discount)."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Displayed to customers on invoices/receipts"},
	{Name: "percent_off", Type: core.ConnectionTypeInteger, Label: "Percent Off", Placeholder: "Percentage discount 1–100 (leave blank for amount off)"},
	{Name: "amount_off", Type: core.ConnectionTypeMoney, Label: "Amount Off", Placeholder: "e.g. 12.34 (requires a currency)"},
	{Name: "currency", Type: core.ConnectionTypeString, Label: "Currency", Placeholder: "Required when Amount Off is set, e.g. gbp"},
	{Name: "duration", Type: core.ConnectionTypeString, Label: "Duration", Value: "once", Options: []core.ConnectionOption{
		{Name: "Once", Value: "once"},
		{Name: "Repeating", Value: "repeating"},
		{Name: "Forever", Value: "forever"},
	}},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata", Placeholder: "Arbitrary key → value pairs stored on the coupon"},
	{Name: "idempotency_key", Type: core.ConnectionTypeString, Label: "Idempotency Key", Placeholder: "Optional — safely retry the same create"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Coupon ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Coupon"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	params := &stripe.CouponParams{}
	if v := stripe_common.OptionalString("name", inputs); v != "" {
		params.Name = stripe.String(v)
	}
	if v := stripe_common.OptionalInt64("percent_off", inputs); v != nil && *v > 0 {
		params.PercentOff = stripe.Float64(float64(*v))
	}
	if v, err := stripe_common.MoneyToMinorUnits("amount_off", stripe_common.OptionalString("currency", inputs), inputs); err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	} else if v != nil && *v > 0 {
		params.AmountOff = v
		if c := stripe_common.OptionalString("currency", inputs); c != "" {
			params.Currency = stripe.String(c)
		}
	}
	duration := stripe_common.OptionalString("duration", inputs)
	if duration == "" {
		duration = "once"
	}
	params.Duration = stripe.String(duration)
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}
	if key := stripe_common.IdempotencyKey(inputs); key != "" {
		params.SetIdempotencyKey(key)
	}

	coupon, err := stripe_common.NewClient(apiKey).Coupons.New(params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(coupon, fmt.Sprintf("Created coupon %s", coupon.ID)), nil
}
