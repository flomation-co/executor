package ecommerce_woocommerce_product_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	woocommerce "flomation.app/automate/executor/actions/ecommerce/woocommerce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WooCommerce: Update Product"
	Description  = "Update an existing product in your WooCommerce store. Only the fields you set are changed; add any other product field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "woocommerce+pen"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Store URL", Placeholder: "https://your-store.com", Required: true},
	{Name: "consumer_key", Type: core.ConnectionTypeSecret, Label: "Consumer Key", Placeholder: "ck_...", Required: true},
	{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Label: "Consumer Secret", Placeholder: "cs_...", Required: true},
	{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Label: "Send Credentials in Query String", Placeholder: "Enable only if you see a \"Consumer key is missing\" error"},
	{Name: "product_id", Type: core.ConnectionTypeString, Label: "Product ID", Placeholder: "123", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "Product name"},
	{
		Name:  "type",
		Type:  core.ConnectionTypeString,
		Label: "Type",
		Options: []core.ConnectionOption{
			{Name: "Simple", Value: "simple"},
			{Name: "Grouped", Value: "grouped"},
			{Name: "External/Affiliate", Value: "external"},
			{Name: "Variable", Value: "variable"},
		},
	},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
		Options: []core.ConnectionOption{
			{Name: "Published", Value: "publish"},
			{Name: "Draft", Value: "draft"},
			{Name: "Pending", Value: "pending"},
			{Name: "Private", Value: "private"},
		},
	},
	{Name: "sku", Type: core.ConnectionTypeString, Label: "SKU"},
	{Name: "regular_price", Type: core.ConnectionTypeString, Label: "Regular Price", Placeholder: "19.99"},
	{Name: "sale_price", Type: core.ConnectionTypeString, Label: "Sale Price"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description"},
	{Name: "short_description", Type: core.ConnectionTypeText, Label: "Short Description"},
	{
		Name:  "catalog_visibility",
		Type:  core.ConnectionTypeString,
		Label: "Catalog Visibility",
		Options: []core.ConnectionOption{
			{Name: "Shop and Search", Value: "visible"},
			{Name: "Shop Only", Value: "catalog"},
			{Name: "Search Only", Value: "search"},
			{Name: "Hidden", Value: "hidden"},
		},
	},
	{
		Name:  "tax_status",
		Type:  core.ConnectionTypeString,
		Label: "Tax Status",
		Options: []core.ConnectionOption{
			{Name: "Taxable", Value: "taxable"},
			{Name: "Shipping Only", Value: "shipping"},
			{Name: "None", Value: "none"},
		},
	},
	{Name: "tax_class", Type: core.ConnectionTypeString, Label: "Tax Class"},
	{
		Name:  "stock_status",
		Type:  core.ConnectionTypeString,
		Label: "Stock Status",
		Options: []core.ConnectionOption{
			{Name: "In Stock", Value: "instock"},
			{Name: "Out of Stock", Value: "outofstock"},
			{Name: "On Backorder", Value: "onbackorder"},
		},
	},
	{
		Name:  "backorders",
		Type:  core.ConnectionTypeString,
		Label: "Backorders",
		Options: []core.ConnectionOption{
			{Name: "Do Not Allow", Value: "no"},
			{Name: "Allow but Notify", Value: "notify"},
			{Name: "Allow", Value: "yes"},
		},
	},
	{Name: "stock_quantity", Type: core.ConnectionTypeInteger, Label: "Stock Quantity"},
	{Name: "weight", Type: core.ConnectionTypeString, Label: "Weight"},
	{Name: "slug", Type: core.ConnectionTypeString, Label: "Slug"},
	{Name: "external_url", Type: core.ConnectionTypeString, Label: "External URL", Placeholder: "For external/affiliate products"},
	{Name: "button_text", Type: core.ConnectionTypeString, Label: "Button Text", Placeholder: "For external/affiliate products"},
	{Name: "purchase_note", Type: core.ConnectionTypeString, Label: "Purchase Note"},
	{Name: "menu_order", Type: core.ConnectionTypeInteger, Label: "Menu Order"},
	{Name: "featured", Type: core.ConnectionTypeBoolean, Label: "Featured"},
	{Name: "virtual", Type: core.ConnectionTypeBoolean, Label: "Virtual"},
	{Name: "downloadable", Type: core.ConnectionTypeBoolean, Label: "Downloadable"},
	{Name: "manage_stock", Type: core.ConnectionTypeBoolean, Label: "Manage Stock"},
	{Name: "sold_individually", Type: core.ConnectionTypeBoolean, Label: "Sold Individually"},
	{Name: "reviews_allowed", Type: core.ConnectionTypeBoolean, Label: "Reviews Allowed"},
	{Name: "category_ids", Type: core.ConnectionTypeString, Label: "Category IDs", Placeholder: "Comma-separated category IDs, e.g. 9,14"},
	{Name: "tag_ids", Type: core.ConnectionTypeString, Label: "Tag IDs", Placeholder: "Comma-separated tag IDs, e.g. 3,7"},
	{Name: "dimensions", Type: core.ConnectionTypeObject, Label: "Dimensions (JSON)", Placeholder: `{"length":"10","width":"5","height":"2"}`},
	{Name: "images", Type: core.ConnectionTypeObject, Label: "Images (JSON)", Placeholder: `[{"src":"https://.../img.jpg"}]`},
	{Name: "attributes", Type: core.ConnectionTypeObject, Label: "Attributes (JSON)", Placeholder: `[{"name":"Color","options":["Red","Blue"],"visible":true}]`},
	{Name: "meta_data", Type: core.ConnectionTypeObject, Label: "Metadata (JSON)", Placeholder: `[{"key":"source","value":"web"}]`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"catalog_visibility":"hidden"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Product ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Product"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := woocommerce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	productID, err := woocommerce.RequiredString("product_id", inputs)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}

	product := map[string]interface{}{}
	woocommerce.SetIfPresent(product, inputs, "name", "name")
	woocommerce.SetIfPresent(product, inputs, "type", "type")
	woocommerce.SetIfPresent(product, inputs, "status", "status")
	woocommerce.SetIfPresent(product, inputs, "sku", "sku")
	woocommerce.SetIfPresent(product, inputs, "regular_price", "regular_price")
	woocommerce.SetIfPresent(product, inputs, "sale_price", "sale_price")
	woocommerce.SetIfPresent(product, inputs, "description", "description")
	woocommerce.SetIfPresent(product, inputs, "short_description", "short_description")
	woocommerce.SetIfPresent(product, inputs, "catalog_visibility", "catalog_visibility")
	woocommerce.SetIfPresent(product, inputs, "tax_status", "tax_status")
	woocommerce.SetIfPresent(product, inputs, "tax_class", "tax_class")
	woocommerce.SetIfPresent(product, inputs, "stock_status", "stock_status")
	woocommerce.SetIfPresent(product, inputs, "backorders", "backorders")
	woocommerce.SetIfPresent(product, inputs, "weight", "weight")
	woocommerce.SetIfPresent(product, inputs, "slug", "slug")
	woocommerce.SetIfPresent(product, inputs, "external_url", "external_url")
	woocommerce.SetIfPresent(product, inputs, "button_text", "button_text")
	woocommerce.SetIfPresent(product, inputs, "purchase_note", "purchase_note")
	woocommerce.SetBoolIfSet(product, inputs, "featured", "featured")
	woocommerce.SetBoolIfSet(product, inputs, "virtual", "virtual")
	woocommerce.SetBoolIfSet(product, inputs, "downloadable", "downloadable")
	woocommerce.SetBoolIfSet(product, inputs, "manage_stock", "manage_stock")
	woocommerce.SetBoolIfSet(product, inputs, "sold_individually", "sold_individually")
	woocommerce.SetBoolIfSet(product, inputs, "reviews_allowed", "reviews_allowed")
	for field, input := range map[string]string{"stock_quantity": "stock_quantity", "menu_order": "menu_order"} {
		if err := woocommerce.SetIntIfPresent(product, inputs, field, input); err != nil {
			return woocommerce.ErrorResult(err.Error()), nil
		}
	}
	woocommerce.SetIDRefsIfPresent(product, inputs, "categories", "category_ids")
	woocommerce.SetIDRefsIfPresent(product, inputs, "tags", "tag_ids")
	for field, input := range map[string]string{
		"dimensions": "dimensions",
		"images":     "images",
		"attributes": "attributes",
		"meta_data":  "meta_data",
	} {
		if err := woocommerce.SetJSONIfPresent(product, inputs, field, input); err != nil {
			return woocommerce.ErrorResult(err.Error()), nil
		}
	}
	if err := woocommerce.MergeAdditionalFields(product, inputs); err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}

	resp, err := woocommerce.UpdateResource(auth, "/products/"+url.PathEscape(productID), product)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	out := woocommerce.ResourceResult(resp, fmt.Sprintf("Updated product %s", productID))
	return out, nil
}
