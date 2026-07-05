package customer_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Customer: Delete"
	Description   = "Permanently delete a Stripe customer."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+trash"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "cus_…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Customer ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Customer"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	id, err := stripe_common.RequiredString("customer_id", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}

	cust, err := stripe_common.NewClient(apiKey).Customers.Del(id, nil)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(cust, fmt.Sprintf("Deleted customer %s", id)), nil
}
