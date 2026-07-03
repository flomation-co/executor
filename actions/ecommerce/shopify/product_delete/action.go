package ecommerce_shopify_product_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	shopify "flomation.app/automate/executor/actions/ecommerce/shopify"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Shopify: Delete Product"
	Description  = "Permanently delete a product from your Shopify store by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "shopify+trash"
	Date         = "02/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "shop", Type: core.ConnectionTypeString, Label: "Shop Subdomain", Placeholder: "my-store (from my-store.myshopify.com)", Required: true},
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Admin API Access Token", Placeholder: "shpat_... — or use Client ID + Secret below"},
	{Name: "client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Dev Dashboard app Client ID (if not using a token)"},
	{Name: "client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "shpss_... — minted into a 24h token automatically"},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product ID", Placeholder: "632910392", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product ID"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	shop, token, err := shopify.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := shopify.RequiredString("product_id", inputs)
	if err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}

	if err := shopify.DeleteResource(shop, token, "/products/"+url.PathEscape(id)+".json"); err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"id":          id,
		"tool_result": fmt.Sprintf("Deleted product %s", id),
		"success":     true,
		"error":       "",
	}, nil
}
