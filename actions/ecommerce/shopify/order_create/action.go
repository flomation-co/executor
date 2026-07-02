package ecommerce_shopify_order_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	shopify "flomation.app/automate/executor/actions/ecommerce/shopify"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Shopify: Create Order"
	Description  = "Create an order in your Shopify store. Provide line items as JSON; set common fields directly or add any other order field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "shopify+plus"
	Date         = "02/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "shop", Type: core.ConnectionTypeString, Label: "Shop Subdomain", Placeholder: "my-store (from my-store.myshopify.com)", Required: true},
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Admin API Access Token", Placeholder: "shpat_... — or use Client ID + Secret below"},
	{Name: "client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Dev Dashboard app Client ID (if not using a token)"},
	{Name: "client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "shpss_... — minted into a 24h token automatically"},
	{
		Name:        "line_items",
		Type:        core.ConnectionTypeObject,
		Label:       "Line Items (JSON)",
		Placeholder: `[{"variant_id":447654529,"quantity":1}]`,
		Required:    true,
	},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Customer Email", Placeholder: "name@email.com"},
	{Name: "note", Type: core.ConnectionTypeText, Label: "Note"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "vip, wholesale (comma-separated)"},
	{
		Name:  "financial_status",
		Type:  core.ConnectionTypeString,
		Label: "Financial Status",
		Options: []core.ConnectionOption{
			{Name: "Pending", Value: "pending"},
			{Name: "Authorized", Value: "authorized"},
			{Name: "Paid", Value: "paid"},
			{Name: "Partially Paid", Value: "partially_paid"},
			{Name: "Refunded", Value: "refunded"},
			{Name: "Partially Refunded", Value: "partially_refunded"},
			{Name: "Voided", Value: "voided"},
		},
	},
	{
		Name:  "fulfillment_status",
		Type:  core.ConnectionTypeString,
		Label: "Fulfillment Status",
		Options: []core.ConnectionOption{
			{Name: "Fulfilled", Value: "fulfilled"},
			{Name: "Partial", Value: "partial"},
			{Name: "Restocked", Value: "restocked"},
		},
	},
	{Name: "billing_address", Type: core.ConnectionTypeObject, Label: "Billing Address (JSON)", Placeholder: `{"first_name":"Jane","last_name":"Doe","address1":"1 Main St","city":"London","country":"GB","zip":"SW1"}`},
	{Name: "shipping_address", Type: core.ConnectionTypeObject, Label: "Shipping Address (JSON)", Placeholder: `{"first_name":"Jane","last_name":"Doe","address1":"1 Main St","city":"London","country":"GB","zip":"SW1"}`},
	{Name: "discount_codes", Type: core.ConnectionTypeObject, Label: "Discount Codes (JSON)", Placeholder: `[{"code":"SAVE10","amount":"10.0","type":"percentage"}]`},
	{Name: "send_receipt", Type: core.ConnectionTypeBoolean, Label: "Send Order Confirmation Email"},
	{Name: "send_fulfillment_receipt", Type: core.ConnectionTypeBoolean, Label: "Send Shipping Confirmation Email"},
	{Name: "test", Type: core.ConnectionTypeBoolean, Label: "Test Order"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"currency":"GBP","source_name":"web"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Order ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Order"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	shop, token, err := shopify.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	lineItems, err := shopify.OptionalJSON("line_items", inputs)
	if err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	if lineItems == nil {
		return shopify.ErrorResult("line_items is required — provide at least one line item as JSON"), nil
	}

	order := map[string]interface{}{"line_items": lineItems}
	shopify.SetIfPresent(order, inputs, "email", "email")
	shopify.SetIfPresent(order, inputs, "note", "note")
	shopify.SetIfPresent(order, inputs, "tags", "tags")
	shopify.SetIfPresent(order, inputs, "financial_status", "financial_status")
	shopify.SetIfPresent(order, inputs, "fulfillment_status", "fulfillment_status")
	shopify.SetBoolIfSet(order, inputs, "send_receipt", "send_receipt")
	shopify.SetBoolIfSet(order, inputs, "send_fulfillment_receipt", "send_fulfillment_receipt")
	shopify.SetBoolIfSet(order, inputs, "test", "test")
	// Iteration order over this map is intentionally unspecified: each field is
	// written independently by SetJSONIfPresent, so the order the JSON fields are
	// applied in has no effect on the resulting order body.
	for field, input := range map[string]string{
		"billing_address":  "billing_address",
		"shipping_address": "shipping_address",
		"discount_codes":   "discount_codes",
	} {
		if err := shopify.SetJSONIfPresent(order, inputs, field, input); err != nil {
			return shopify.ErrorResult(err.Error()), nil
		}
	}
	if err := shopify.MergeAdditionalFields(order, inputs); err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}

	resp, err := shopify.CreateResource(shop, token, "/orders.json", "order", order)
	if err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	out := shopify.ResourceResult(resp, "order", "")
	out["tool_result"] = fmt.Sprintf("Created order %s", out["id"])
	return out, nil
}
