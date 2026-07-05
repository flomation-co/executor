package invoice_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Invoice: Create"
	Description  = "Create a draft Stripe invoice for a customer. Returns the invoice ID and object."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "customer", Type: core.ConnectionTypeString, Label: "Customer", Placeholder: "cus_…", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "auto_advance", Type: core.ConnectionTypeBoolean, Label: "Auto Advance", Placeholder: "Automatically finalise and attempt collection"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata", Placeholder: "Arbitrary key → value pairs stored on the invoice"},
	{Name: "idempotency_key", Type: core.ConnectionTypeString, Label: "Idempotency Key", Placeholder: "Optional — safely retry the same create"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Invoice ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Invoice"},
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

	params := &stripe.InvoiceParams{}
	params.Customer = stripe.String(customer)
	if v := stripe_common.OptionalString("description", inputs); v != "" {
		params.Description = stripe.String(v)
	}
	if v := stripe_common.OptionalBool("auto_advance", inputs); v != nil {
		params.AutoAdvance = stripe.Bool(*v)
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}
	if key := stripe_common.IdempotencyKey(inputs); key != "" {
		params.SetIdempotencyKey(key)
	}

	inv, err := stripe_common.NewClient(apiKey).Invoices.New(params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(inv, fmt.Sprintf("Created invoice %s", inv.ID)), nil
}
