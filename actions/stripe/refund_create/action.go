package refund_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Refund: Create"
	Description  = "Refund a Stripe charge or payment intent, in full or part."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "payment_intent", Type: core.ConnectionTypeString, Label: "Payment Intent", Placeholder: "pi_…"},
	{Name: "charge", Type: core.ConnectionTypeString, Label: "Charge", Placeholder: "ch_…"},
	{Name: "amount", Type: core.ConnectionTypeMoney, Label: "Amount", Placeholder: "e.g. 12.34 — omit for a full refund"},
	{Name: "reason", Type: core.ConnectionTypeString, Label: "Reason", Placeholder: "duplicate, fraudulent or requested_by_customer"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata", Placeholder: "Arbitrary key → value pairs stored on the refund"},
	{Name: "idempotency_key", Type: core.ConnectionTypeString, Label: "Idempotency Key", Placeholder: "Optional — safely retry the same create"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Refund ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Refund"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	params := &stripe.RefundParams{}
	if v := stripe_common.OptionalString("payment_intent", inputs); v != "" {
		params.PaymentIntent = stripe.String(v)
	}
	if v := stripe_common.OptionalString("charge", inputs); v != "" {
		params.Charge = stripe.String(v)
	}
	if v, err := stripe_common.MoneyToMinorUnits("amount", "", inputs); err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	} else if v != nil && *v > 0 {
		params.Amount = stripe.Int64(*v)
	}
	if v := stripe_common.OptionalString("reason", inputs); v != "" {
		params.Reason = stripe.String(v)
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}
	if key := stripe_common.IdempotencyKey(inputs); key != "" {
		params.SetIdempotencyKey(key)
	}

	refund, err := stripe_common.NewClient(apiKey).Refunds.New(params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(refund, fmt.Sprintf("Created refund %s", refund.ID)), nil
}
