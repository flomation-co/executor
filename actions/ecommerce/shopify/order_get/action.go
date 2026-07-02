package ecommerce_shopify_order_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	shopify "flomation.app/automate/executor/actions/ecommerce/shopify"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Shopify: Get Order"
	Description  = "Retrieve a single order from your Shopify store by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "shopify+search"
	Date         = "02/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "shop", Type: core.ConnectionTypeString, Label: "Shop Subdomain", Placeholder: "my-store (from my-store.myshopify.com)", Required: true},
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Admin API Access Token", Placeholder: "shpat_... — or use Client ID + Secret below"},
	{Name: "client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Dev Dashboard app Client ID (if not using a token)"},
	{Name: "client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "shpss_... — minted into a 24h token automatically"},
	{Name: "order_id", Type: core.ConnectionTypeString, Label: "Order ID", Placeholder: "450789469", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return (optional)"},
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

	q := url.Values{}
	shopify.AddFilter(q, inputs, "fields", "fields")

	resp, err := shopify.GetResource(shop, token, "/orders/"+url.PathEscape(id)+".json", q)
	if err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	out := shopify.ResourceResult(resp, "order", fmt.Sprintf("Retrieved order %s", id))
	return out, nil
}
