package ecommerce_shopify_product_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	shopify "flomation.app/automate/executor/actions/ecommerce/shopify"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Shopify: Create Product"
	Description  = "Create a product in your Shopify store. Set common fields directly; supply variants, images, and options as JSON, or any other field via Additional Fields."
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
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "Burton Custom Freestyle 151", Required: true},
	{Name: "price", Type: core.ConnectionTypeString, Label: "Price", Placeholder: "19.99 (sets the default variant's price)"},
	{Name: "sku", Type: core.ConnectionTypeString, Label: "SKU", Placeholder: "SB-001"},
	{Name: "body_html", Type: core.ConnectionTypeText, Label: "Description (HTML)", Placeholder: "<strong>Great snowboard!</strong>"},
	{Name: "vendor", Type: core.ConnectionTypeString, Label: "Vendor", Placeholder: "Burton"},
	{Name: "product_type", Type: core.ConnectionTypeString, Label: "Product Type", Placeholder: "Snowboard"},
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
	{Name: "handle", Type: core.ConnectionTypeString, Label: "Handle", Placeholder: "burton-custom (auto-generated from title if blank)"},
	{Name: "variants", Type: core.ConnectionTypeObject, Label: "Variants (JSON)", Placeholder: `[{"option1":"Small","price":"19.99","sku":"SB-S"}]`},
	{Name: "options", Type: core.ConnectionTypeObject, Label: "Options (JSON)", Placeholder: `[{"name":"Size","values":["Small","Medium"]}]`},
	{Name: "images", Type: core.ConnectionTypeObject, Label: "Images (JSON)", Placeholder: `[{"src":"https://example.com/img.jpg"}]`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"template_suffix":"special"}`},
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
	title, err := shopify.RequiredString("title", inputs)
	if err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}

	product := map[string]interface{}{"title": title}
	shopify.SetIfPresent(product, inputs, "body_html", "body_html")
	shopify.SetIfPresent(product, inputs, "vendor", "vendor")
	shopify.SetIfPresent(product, inputs, "product_type", "product_type")
	shopify.SetIfPresent(product, inputs, "tags", "tags")
	shopify.SetIfPresent(product, inputs, "status", "status")
	shopify.SetIfPresent(product, inputs, "handle", "handle")
	// Iteration order over this map is intentionally unspecified: each field is
	// written independently by SetJSONIfPresent, so the order the three JSON
	// fields are applied in has no effect on the resulting product body.
	for field, input := range map[string]string{
		"variants": "variants",
		"options":  "options",
		"images":   "images",
	} {
		if err := shopify.SetJSONIfPresent(product, inputs, field, input); err != nil {
			return shopify.ErrorResult(err.Error()), nil
		}
	}
	// When no explicit Variants JSON was given, build a default variant from
	// the simple Price/SKU fields so a basic product just needs a title+price.
	if _, ok := product["variants"]; !ok {
		if dv := shopify.BuildDefaultVariant(inputs); dv != nil {
			product["variants"] = dv
		}
	}
	if err := shopify.MergeAdditionalFields(product, inputs); err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}

	resp, err := shopify.CreateResource(shop, token, "/products.json", "product", product)
	if err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	out := shopify.ResourceResult(resp, "product", "")
	out["tool_result"] = fmt.Sprintf("Created product %s", out["id"])
	return out, nil
}
