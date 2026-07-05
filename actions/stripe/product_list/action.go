package product_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Product: List"
	Description  = "List Stripe products, optionally filtered by active state."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "active", Type: core.ConnectionTypeBoolean, Label: "Active", Placeholder: "Filter by active state (optional)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Max results (default 20, max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Products"},
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
	params := &stripe.ProductListParams{}
	params.Limit = stripe.Int64(limit)
	if b := stripe_common.OptionalBool("active", inputs); b != nil {
		params.Active = b
	}

	it := stripe_common.NewClient(apiKey).Products.List(params)
	var items []map[string]interface{}
	for it.Next() {
		items = append(items, stripe_common.ToMap(it.Product()))
	}
	if err := it.Err(); err != nil {
		return stripe_common.MapError(err), nil
	}
	hasMore := it.Meta() != nil && it.Meta().HasMore
	return stripe_common.ListResult(items, hasMore, fmt.Sprintf("Listed %d product(s)", len(items))), nil
}
