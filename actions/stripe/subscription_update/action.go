package subscription_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Subscription: Update"
	Description  = "Update a Stripe subscription (cancel at period end, metadata)."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "subscription_id", Type: core.ConnectionTypeString, Label: "Subscription ID", Placeholder: "sub_…", Required: true},
	{Name: "cancel_at_period_end", Type: core.ConnectionTypeBoolean, Label: "Cancel At Period End"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata"},
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
	id, err := stripe_common.RequiredString("subscription_id", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}

	params := &stripe.SubscriptionParams{}
	if b := stripe_common.OptionalBool("cancel_at_period_end", inputs); b != nil {
		params.CancelAtPeriodEnd = b
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}

	sub, err := stripe_common.NewClient(apiKey).Subscriptions.Update(id, params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(sub, fmt.Sprintf("Updated subscription %s", sub.ID)), nil
}
