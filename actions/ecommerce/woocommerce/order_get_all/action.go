package ecommerce_woocommerce_order_get_all

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
	Name         = "WooCommerce: Get Many Orders"
	Description  = "List orders from your WooCommerce store, with optional filters. Enable Return All to auto-paginate every matching order."
	Website      = "https://www.flomation.co"
	Icon         = "woocommerce+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Store URL", Placeholder: "https://your-store.com — your store's root URL, not the /wp-json path", Required: true},
	{Name: "consumer_key", Type: core.ConnectionTypeSecret, Label: "Consumer Key", Placeholder: "ck_...", Required: true},
	{Name: "consumer_secret", Type: core.ConnectionTypeSecret, Label: "Consumer Secret", Placeholder: "cs_...", Required: true},
	{Name: "credentials_in_query", Type: core.ConnectionTypeBoolean, Label: "Send Credentials in Query String", Placeholder: "Enable only if you see a \"Consumer key is missing\" error"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (auto-paginate every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit (per page)", Placeholder: "50 per page (max 100); Return All still fetches every match"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Page number to fetch (ignored when Return All is on)"},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
		Options: []core.ConnectionOption{
			{Name: "Any", Value: "any"},
			{Name: "Pending Payment", Value: "pending"},
			{Name: "Processing", Value: "processing"},
			{Name: "On Hold", Value: "on-hold"},
			{Name: "Completed", Value: "completed"},
			{Name: "Cancelled", Value: "cancelled"},
			{Name: "Refunded", Value: "refunded"},
			{Name: "Failed", Value: "failed"},
			{Name: "Trash", Value: "trash"},
		},
	},
	{Name: "customer", Type: core.ConnectionTypeString, Label: "Customer ID", Placeholder: "Limit to orders for this customer ID"},
	{Name: "product", Type: core.ConnectionTypeString, Label: "Product ID", Placeholder: "Limit to orders containing this product ID"},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search"},
	{Name: "after", Type: core.ConnectionTypeDateTime, Label: "Created After"},
	{Name: "before", Type: core.ConnectionTypeDateTime, Label: "Created Before"},
	{Name: "include", Type: core.ConnectionTypeString, Label: "Include IDs", Placeholder: "Comma-separated order IDs to include"},
	{Name: "exclude", Type: core.ConnectionTypeString, Label: "Exclude IDs", Placeholder: "Comma-separated order IDs to exclude"},
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
		},
	},
	{Name: "dp", Type: core.ConnectionTypeInteger, Label: "Decimal Points", Placeholder: "Number of decimal points in prices (default 2)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Orders"},
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
	woocommerce.AddFilter(q, inputs, "status", "status")
	woocommerce.AddFilter(q, inputs, "customer", "customer")
	woocommerce.AddFilter(q, inputs, "product", "product")
	woocommerce.AddFilter(q, inputs, "search", "search")
	woocommerce.AddFilter(q, inputs, "after", "after")
	woocommerce.AddFilter(q, inputs, "before", "before")
	woocommerce.AddFilter(q, inputs, "include", "include")
	woocommerce.AddFilter(q, inputs, "exclude", "exclude")
	woocommerce.AddFilter(q, inputs, "order", "order")
	woocommerce.AddFilter(q, inputs, "orderby", "orderby")
	if dp, ok := woocommerce.OptionalInt("dp", inputs); ok {
		q.Set("dp", strconv.Itoa(dp))
	}

	items, total, totalPages, pages, err := woocommerce.ListResources(auth, "/orders", q, returnAll)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	out := woocommerce.ListResult(items, total, totalPages, "")
	if returnAll && pages >= woocommerce.MaxAllPages && totalPages > pages {
		out["tool_result"] = fmt.Sprintf("Fetched %d order(s) across %d page(s); stopped at the %d-page safety cap — narrow the filters or page through manually to get the rest", len(items), pages, woocommerce.MaxAllPages)
	} else if returnAll {
		out["tool_result"] = fmt.Sprintf("Fetched all %d order(s) across %d page(s)", len(items), pages)
	} else {
		out["tool_result"] = fmt.Sprintf("Found %d order(s)", len(items))
	}
	return out, nil
}
