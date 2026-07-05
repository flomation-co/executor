package customer_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Customer: Update"
	Description   = "Update fields on an existing Stripe customer."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "customer_id", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "cus_…", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata"},
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

	params := &stripe.CustomerParams{}
	if v := stripe_common.OptionalString("email", inputs); v != "" {
		params.Email = stripe.String(v)
	}
	if v := stripe_common.OptionalString("name", inputs); v != "" {
		params.Name = stripe.String(v)
	}
	if v := stripe_common.OptionalString("phone", inputs); v != "" {
		params.Phone = stripe.String(v)
	}
	if v := stripe_common.OptionalString("description", inputs); v != "" {
		params.Description = stripe.String(v)
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}

	cust, err := stripe_common.NewClient(apiKey).Customers.Update(id, params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(cust, fmt.Sprintf("Updated customer %s", cust.ID)), nil
}
