package ecommerce_woocommerce_product_get_all

import (
	"fmt"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	woocommerce "flomation.app/automate/executor/actions/ecommerce/woocommerce"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WooCommerce: Get Many Products"
	Description  = "List products from your WooCommerce store, with optional filters. Enable Return All to auto-paginate every matching product."
	Website      = "https://www.flomation.co"
	Icon         = "woocommerce+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Store URL", Placeholder: "https://your-store.com", Required: true},
	{Name: "consumer_key", Type: core.ConnectionTypeSecret, Label: "Consumer Key", Placeholder: "ck_...", Required: true},
	{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Label: "Consumer Secret", Placeholder: "cs_...", Required: true},
	{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Label: "Send Credentials in Query String", Placeholder: "Enable only if you see a \"Consumer key is missing\" error"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (auto-paginate every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit (per page)", Placeholder: "50 per page (max 100); Return All still fetches every match"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Page number to fetch (ignored when Return All is on)"},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search"},
	{Name: "after", Type: core.ConnectionTypeDateTime, Label: "Created After"},
	{Name: "before", Type: core.ConnectionTypeDateTime, Label: "Created Before"},
	{Name: "sku", Type: core.ConnectionTypeString, Label: "SKU"},
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
			{Name: "Any", Value: "any"},
			{Name: "Published", Value: "publish"},
			{Name: "Draft", Value: "draft"},
			{Name: "Pending", Value: "pending"},
			{Name: "Private", Value: "private"},
		},
	},
	{Name: "category", Type: core.ConnectionTypeString, Label: "Category ID", Placeholder: "Limit to products in this category ID"},
	{Name: "tag", Type: core.ConnectionTypeString, Label: "Tag ID", Placeholder: "Limit to products with this tag ID"},
	{Name: "featured", Type: core.ConnectionTypeBoolean, Label: "Featured Only"},
	{Name: "on_sale", Type: core.ConnectionTypeBoolean, Label: "On Sale Only"},
	{Name: "min_price", Type: core.ConnectionTypeString, Label: "Minimum Price"},
	{Name: "max_price", Type: core.ConnectionTypeString, Label: "Maximum Price"},
	{
		Name:  "stock_status",
		Type:  core.ConnectionTypeString,
		Label: "Stock Status",
		Options: []core.ConnectionOption{
			{Name: "Any", Value: ""},
			{Name: "In Stock", Value: "instock"},
			{Name: "Out of Stock", Value: "outofstock"},
			{Name: "On Backorder", Value: "onbackorder"},
		},
	},
	{
		Name:  "order",
		Type:  core.ConnectionTypeString,
		Label: "Sort Order",
		Options: []core.ConnectionOption{
			{Name: "Descending", Value: "desc"},
			{Name: "Ascending", Value: "asc"},
		},
	},
	{
		Name:  "orderby",
		Type:  core.ConnectionTypeString,
		Label: "Order By",
		Options: []core.ConnectionOption{
			{Name: "Date", Value: "date"},
			{Name: "ID", Value: "id"},
			{Name: "Include", Value: "include"},
			{Name: "Title", Value: "title"},
			{Name: "Slug", Value: "slug"},
			{Name: "Price", Value: "price"},
			{Name: "Popularity", Value: "popularity"},
			{Name: "Rating", Value: "rating"},
		},
	},
	{Name: "include", Type: core.ConnectionTypeString, Label: "Include IDs", Placeholder: "Comma-separated product IDs to include"},
	{Name: "exclude", Type: core.ConnectionTypeString, Label: "Exclude IDs", Placeholder: "Comma-separated product IDs to exclude"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset", Placeholder: "Skip this many results"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Products"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count (this response)"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total (store-wide match)"},
	{Name: "total_pages", Type: core.ConnectionTypeInteger, Label: "Total Pages"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := woocommerce.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	returnAll := woocommerce.OptionalBool("return_all", inputs)
	q := url.Values{}
	limit, set := woocommerce.OptionalInt("limit", inputs)
	q.Set("per_page", strconv.Itoa(woocommerce.ClampLimit(limit, set)))
	if !returnAll {
		if page, ok := woocommerce.OptionalInt("page", inputs); ok && page > 0 {
			q.Set("page", strconv.Itoa(page))
		}
	}
	woocommerce.AddFilter(q, inputs, "search", "search")
	woocommerce.AddFilter(q, inputs, "after", "after")
	woocommerce.AddFilter(q, inputs, "before", "before")
	woocommerce.AddFilter(q, inputs, "sku", "sku")
	woocommerce.AddFilter(q, inputs, "type", "type")
	woocommerce.AddFilter(q, inputs, "status", "status")
	woocommerce.AddFilter(q, inputs, "category", "category")
	woocommerce.AddFilter(q, inputs, "tag", "tag")
	woocommerce.AddFilter(q, inputs, "min_price", "min_price")
	woocommerce.AddFilter(q, inputs, "max_price", "max_price")
	woocommerce.AddFilter(q, inputs, "stock_status", "stock_status")
	woocommerce.AddFilter(q, inputs, "order", "order")
	woocommerce.AddFilter(q, inputs, "orderby", "orderby")
	woocommerce.AddFilter(q, inputs, "include", "include")
	woocommerce.AddFilter(q, inputs, "exclude", "exclude")
	if woocommerce.OptionalBool("featured", inputs) {
		q.Set("featured", "true")
	}
	if woocommerce.OptionalBool("on_sale", inputs) {
		q.Set("on_sale", "true")
	}
	if offset, ok := woocommerce.OptionalInt("offset", inputs); ok {
		q.Set("offset", strconv.Itoa(offset))
	}

	items, total, totalPages, pages, err := woocommerce.ListResources(auth, "/products", q, returnAll)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	out := woocommerce.ListResult(items, total, totalPages, "")
	if returnAll && pages >= woocommerce.MaxAllPages && totalPages > pages {
		out["tool_result"] = fmt.Sprintf("Fetched %d product(s) across %d page(s); stopped at the %d-page safety cap — narrow the filters or page through manually to get the rest", len(items), pages, woocommerce.MaxAllPages)
	} else if returnAll {
		out["tool_result"] = fmt.Sprintf("Fetched all %d product(s) across %d page(s)", len(items), pages)
	} else {
		out["tool_result"] = fmt.Sprintf("Found %d product(s)", len(items))
	}
	return out, nil
}
