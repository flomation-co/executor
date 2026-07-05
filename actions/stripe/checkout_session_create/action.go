package checkout_session_create

import (
	"fmt"
	"strconv"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Checkout Session: Create"
	Description  = "Create a Stripe Checkout Session and return its hosted payment URL."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "mode", Type: core.ConnectionTypeString, Label: "Mode", Placeholder: "payment", Options: []core.ConnectionOption{
		{Name: "Payment", Value: "payment"},
		{Name: "Subscription", Value: "subscription"},
		{Name: "Setup", Value: "setup"},
	}},
	{Name: "success_url", Type: core.ConnectionTypeString, Label: "Success URL", Placeholder: "https://example.com/success", Required: true},
	{Name: "cancel_url", Type: core.ConnectionTypeString, Label: "Cancel URL", Placeholder: "https://example.com/cancel"},
	{Name: "customer", Type: core.ConnectionTypeString, Label: "Customer", Placeholder: "cus_…"},
	{Name: "line_items", Type: core.ConnectionTypeKeyValueArray, Label: "Line Items", Placeholder: "price_id → quantity"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Session ID"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Checkout URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Checkout Session"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	successURL, err := stripe_common.RequiredString("success_url", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}

	mode := stripe_common.OptionalString("mode", inputs)
	if mode == "" {
		mode = "payment"
	}

	params := &stripe.CheckoutSessionParams{
		Mode:       stripe.String(mode),
		SuccessURL: stripe.String(successURL),
	}
	if v := stripe_common.OptionalString("cancel_url", inputs); v != "" {
		params.CancelURL = stripe.String(v)
	}
	if v := stripe_common.OptionalString("customer", inputs); v != "" {
		params.Customer = stripe.String(v)
	}

	if c := core.FindConnection("line_items", inputs); c != nil {
		for _, kv := range c.KeyValuePairs() {
			if kv.Key == "" {
				continue
			}
			qty := int64(1)
			if kv.Value != "" {
				if parsed, perr := strconv.Atoi(kv.Value); perr == nil {
					qty = int64(parsed)
				}
			}
			params.LineItems = append(params.LineItems, &stripe.CheckoutSessionLineItemParams{
				Price:    stripe.String(kv.Key),
				Quantity: stripe.Int64(qty),
			})
		}
	}

	sess, err := stripe_common.NewClient(apiKey).CheckoutSessions.New(params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	result := stripe_common.ObjectResult(sess, fmt.Sprintf("Created checkout session %s", sess.ID))
	result["url"] = sess.URL
	return result, nil
}
