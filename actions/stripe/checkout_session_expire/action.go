package checkout_session_expire

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Checkout Session: Expire"
	Description  = "Expire an open Stripe Checkout Session so it can no longer be completed."
	Website      = "https://www.flomation.co"
	Icon         = "stripe"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "session_id", Type: core.ConnectionTypeString, Label: "Session ID", Placeholder: "cs_…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Session ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Checkout Session"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	id, err := stripe_common.RequiredString("session_id", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}

	sess, err := stripe_common.NewClient(apiKey).CheckoutSessions.Expire(id, &stripe.CheckoutSessionExpireParams{})
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(sess, fmt.Sprintf("Expired checkout session %s", sess.ID)), nil
}
