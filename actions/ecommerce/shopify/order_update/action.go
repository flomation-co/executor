package ecommerce_shopify_order_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	shopify "flomation.app/automate/executor/actions/ecommerce/shopify"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Shopify: Update Order"
	Description  = "Update an existing order in your Shopify store. Only the fields you set are changed."
	Website      = "https://www.flomation.co"
	Icon         = "shopify+pencil"
	Date         = "02/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "shop", Type: core.ConnectionTypeString, Label: "Shop Subdomain", Placeholder: "my-store (from my-store.myshopify.com)", Required: true},
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Admin API Access Token", Placeholder: "shpat_... — or use Client ID + Secret below"},
	{Name: "client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Dev Dashboard app Client ID (if not using a token)"},
	{Name: "client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "shpss_... — minted into a 24h token automatically"},
	{Name: "order_id", Type: core.ConnectionTypeString, Label: "Order ID", Placeholder: "450789469", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Customer Email", Placeholder: "name@email.com"},
	{Name: "note", Type: core.ConnectionTypeText, Label: "Note"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "vip, wholesale (comma-separated)"},
	{Name: "shipping_address", Type: core.ConnectionTypeObject, Label: "Shipping Address (JSON)", Placeholder: `{"address1":"1 Main St","city":"London","country":"GB","zip":"SW1"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"buyer_accepts_marketing":true}`},
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
	id, err := shopify.RequiredString("order_id", inputs)
	if err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}

	order := map[string]interface{}{"id": id}
	shopify.SetIfPresent(order, inputs, "email", "email")
	shopify.SetIfPresent(order, inputs, "note", "note")
	shopify.SetIfPresent(order, inputs, "tags", "tags")
	if err := shopify.SetJSONIfPresent(order, inputs, "shipping_address", "shipping_address"); err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	if err := shopify.MergeAdditionalFields(order, inputs); err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	if len(order) == 1 {
		return shopify.ErrorResult("no fields to update — set at least one field"), nil
	}

	resp, err := shopify.UpdateResource(shop, token, "/orders/"+url.PathEscape(id)+".json", "order", order)
	if err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	out := shopify.ResourceResult(resp, "order", fmt.Sprintf("Updated order %s", id))
	return out, nil
}
