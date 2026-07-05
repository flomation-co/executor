package price_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Price: Update"
	Description  = "Update mutable fields (active, nickname, metadata) on a Stripe price."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "price_id", Type: core.ConnectionTypeString, Label: "Price ID", Placeholder: "price_…", Required: true},
	{Name: "active", Type: core.ConnectionTypeBoolean, Label: "Active"},
	{Name: "nickname", Type: core.ConnectionTypeString, Label: "Nickname"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata"},
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
	id, err := stripe_common.RequiredString("price_id", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}

	params := &stripe.PriceParams{}
	if b := stripe_common.OptionalBool("active", inputs); b != nil {
		params.Active = b
	}
	if v := stripe_common.OptionalString("nickname", inputs); v != "" {
		params.Nickname = stripe.String(v)
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}

	price, err := stripe_common.NewClient(apiKey).Prices.Update(id, params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(price, fmt.Sprintf("Updated price %s", price.ID)), nil
}
