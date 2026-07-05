package subscription_create

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
	Name         = "Subscription: Create"
	Description  = "Create a Stripe subscription for a customer from price/quantity items."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "customer", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "cus_…", Required: true},
	{Name: "line_items", Type: core.ConnectionTypeKeyValueArray, Label: "Line Items", Placeholder: "Price ID → quantity"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata", Placeholder: "Arbitrary key → value pairs stored on the subscription"},
	{Name: "idempotency_key", Type: core.ConnectionTypeString, Label: "Idempotency Key", Placeholder: "Optional — safely retry the same create"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Subscription ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Subscription"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	customer, err := stripe_common.RequiredString("customer", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}

	var items []*stripe.SubscriptionItemsParams
	if c := core.FindConnection("line_items", inputs); c != nil {
		for _, kv := range c.KeyValuePairs() {
			if kv.Key == "" {
				continue
			}
			qty := int64(1)
			if n, perr := strconv.Atoi(kv.Value); perr == nil && n > 0 {
				qty = int64(n)
			}
			items = append(items, &stripe.SubscriptionItemsParams{
				Price:    stripe.String(kv.Key),
				Quantity: stripe.Int64(qty),
			})
		}
	}

	params := &stripe.SubscriptionParams{
		Customer: stripe.String(customer),
		Items:    items,
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}
	if key := stripe_common.IdempotencyKey(inputs); key != "" {
		params.SetIdempotencyKey(key)
	}

	sub, err := stripe_common.NewClient(apiKey).Subscriptions.New(params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(sub, fmt.Sprintf("Created subscription %s", sub.ID)), nil
}
