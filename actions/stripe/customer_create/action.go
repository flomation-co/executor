package customer_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Customer: Create"
	Description   = "Create a Stripe customer. Returns the customer ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "customer@example.com"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Ada Lovelace"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "+44 20 7946 0000"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata", Placeholder: "Arbitrary key → value pairs stored on the customer"},
	{Name: "idempotency_key", Type: core.ConnectionTypeString, Label: "Idempotency Key", Placeholder: "Optional — safely retry the same create"},
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
	if key := stripe_common.IdempotencyKey(inputs); key != "" {
		params.SetIdempotencyKey(key)
	}

	cust, err := stripe_common.NewClient(apiKey).Customers.New(params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(cust, fmt.Sprintf("Created customer %s", cust.ID)), nil
}
