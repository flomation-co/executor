package promotion_code_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	stripe_common "flomation.app/automate/executor/actions/stripe"
	stripe "github.com/stripe/stripe-go/v82"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Promotion Code: Create"
	Description  = "Create a customer-facing promotion code for a Stripe coupon."
	Website      = "https://www.flomation.co"
	Icon         = "stripe+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Stripe Secret Key", Placeholder: "sk_live_… or sk_test_…", Required: true},
	{Name: "coupon", Type: core.ConnectionTypeString, Label: "Coupon ID", Placeholder: "The coupon this code applies", Required: true},
	{Name: "code", Type: core.ConnectionTypeString, Label: "Code", Placeholder: "Customer-facing code (leave blank to auto-generate)"},
	{Name: "active", Type: core.ConnectionTypeBoolean, Label: "Active", Placeholder: "Whether the promotion code is currently active"},
	{Name: "metadata", Type: core.ConnectionTypeKeyValueArray, Label: "Metadata", Placeholder: "Arbitrary key → value pairs stored on the promotion code"},
	{Name: "idempotency_key", Type: core.ConnectionTypeString, Label: "Idempotency Key", Placeholder: "Optional — safely retry the same create"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Promotion Code ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Promotion Code"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := stripe_common.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}
	coupon, err := stripe_common.RequiredString("coupon", inputs)
	if err != nil {
		return stripe_common.ErrorResult(err.Error()), nil
	}

	params := &stripe.PromotionCodeParams{}
	params.Coupon = stripe.String(coupon)
	if v := stripe_common.OptionalString("code", inputs); v != "" {
		params.Code = stripe.String(v)
	}
	if v := stripe_common.OptionalBool("active", inputs); v != nil {
		params.Active = v
	}
	for k, v := range stripe_common.Metadata(inputs) {
		params.AddMetadata(k, v)
	}
	if key := stripe_common.IdempotencyKey(inputs); key != "" {
		params.SetIdempotencyKey(key)
	}

	pc, err := stripe_common.NewClient(apiKey).PromotionCodes.New(params)
	if err != nil {
		return stripe_common.MapError(err), nil
	}
	return stripe_common.ObjectResult(pc, fmt.Sprintf("Created promotion code %s", pc.ID)), nil
}
