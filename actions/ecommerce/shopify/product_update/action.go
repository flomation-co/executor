package ecommerce_shopify_product_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	shopify "flomation.app/automate/executor/actions/ecommerce/shopify"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Shopify: Update Product"
	Description  = "Update an existing product in your Shopify store. Only the fields you set are changed."
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
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product ID", Placeholder: "632910392", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "body_html", Type: core.ConnectionTypeText, Label: "Description (HTML)"},
	{Name: "vendor", Type: core.ConnectionTypeString, Label: "Vendor"},
	{Name: "product_type", Type: core.ConnectionTypeString, Label: "Product Type"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tags", Placeholder: "winter, sale (comma-separated)"},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
		Options: []core.ConnectionOption{
			{Name: "Active", Value: "active"},
			{Name: "Draft", Value: "draft"},
			{Name: "Archived", Value: "archived"},
		},
	},
	{Name: "handle", Type: core.ConnectionTypeString, Label: "Handle"},
	{Name: "variants", Type: core.ConnectionTypeObject, Label: "Variants (JSON)", Placeholder: `[{"id":808950810,"price":"24.99"}]`},
	{Name: "options", Type: core.ConnectionTypeObject, Label: "Options (JSON)"},
	{Name: "images", Type: core.ConnectionTypeObject, Label: "Images (JSON)", Placeholder: `[{"src":"https://example.com/img.jpg"}]`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Product"},
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

	product := map[string]interface{}{"id": id}
	shopify.SetIfPresent(product, inputs, "title", "title")
	shopify.SetIfPresent(product, inputs, "body_html", "body_html")
	shopify.SetIfPresent(product, inputs, "vendor", "vendor")
	shopify.SetIfPresent(product, inputs, "product_type", "product_type")
	shopify.SetIfPresent(product, inputs, "tags", "tags")
	shopify.SetIfPresent(product, inputs, "status", "status")
	shopify.SetIfPresent(product, inputs, "handle", "handle")
	for field, input := range map[string]string{
		"variants": "variants",
		"options":  "options",
		"images":   "images",
	} {
		if err := shopify.SetJSONIfPresent(product, inputs, field, input); err != nil {
			return shopify.ErrorResult(err.Error()), nil
		}
	}
	if err := shopify.MergeAdditionalFields(product, inputs); err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	if len(product) == 1 {
		return shopify.ErrorResult("no fields to update — set at least one field"), nil
	}

	resp, err := shopify.UpdateResource(shop, token, "/products/"+url.PathEscape(id)+".json", "product", product)
	if err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	out := shopify.ResourceResult(resp, "product", fmt.Sprintf("Updated product %s", id))
	return out, nil
}
