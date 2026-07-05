package dispute_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Dispute: List"
	Description  = "List Stripe disputes, optionally filtered by charge or payment intent."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "charge", Type: core.ConnectionTypeString, Label: "Charge ID", Placeholder: "Filter by charge (optional)"},
	{Name: "payment_intent", Type: core.ConnectionTypeString, Label: "Payment Intent ID", Placeholder: "Filter by payment intent (optional)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max results (default 20, max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Disputes"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "has_more", Type: core.ConnectionTypeBoolean, Label: "Has More"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	limit := int64(20)
	if l := stripe_common.OptionalInt64("limit", inputs); l != nil && *l > 0 {
		limit = *l
	}
	params := &stripe.DisputeListParams{}
	params.Limit = stripe.Int64(limit)
	if v := stripe_common.OptionalString("charge", inputs); v != "" {
		params.Charge = stripe.String(v)
	}
	if v := stripe_common.OptionalString("payment_intent", inputs); v != "" {
		params.PaymentIntent = stripe.String(v)
	}

	it := stripe_common.NewClient(apiKey).Disputes.List(params)
	var items []map[string]interface{}
	for it.Next() {
		items = append(items, stripe_common.ToMap(it.Dispute()))
	}
	if err := it.Err(); err != nil {
		return stripe_common.MapError(err), nil
	}
	hasMore := it.Meta() != nil && it.Meta().HasMore
	return stripe_common.ListResult(items, hasMore, fmt.Sprintf("Listed %d dispute(s)", len(items))), nil
}
