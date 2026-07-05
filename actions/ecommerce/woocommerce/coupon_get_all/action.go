package ecommerce_woocommerce_coupon_get_all

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
	Name         = "WooCommerce: Get Many Coupons"
	Description  = "List coupons from your WooCommerce store, with optional filters. Enable Return All to auto-paginate every matching coupon."
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
	{Name: "code", Type: core.ConnectionTypeString, Label: "Coupon Code", Placeholder: "Limit to the coupon with this exact code"},
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
	{Name: "include", Type: core.ConnectionTypeString, Label: "Include IDs", Placeholder: "Comma-separated coupon IDs to include"},
	{Name: "exclude", Type: core.ConnectionTypeString, Label: "Exclude IDs", Placeholder: "Comma-separated coupon IDs to exclude"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset", Placeholder: "Number of results to skip (ignored when Return All is on)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Coupons"},
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
	woocommerce.AddFilter(q, inputs, "code", "code")
	woocommerce.AddFilter(q, inputs, "order", "order")
	woocommerce.AddFilter(q, inputs, "orderby", "orderby")
	woocommerce.AddFilter(q, inputs, "include", "include")
	woocommerce.AddFilter(q, inputs, "exclude", "exclude")
	// Offset is a single-page positioning param; WooCommerce lets it override the
	// page cursor, so applying it during a Return All run would refetch the same
	// window every page (duplicated results). Only honour it for a single page.
	if offset, ok := woocommerce.OptionalInt("offset", inputs); ok && !returnAll {
		q.Set("offset", strconv.Itoa(offset))
	}

	items, total, totalPages, pages, err := woocommerce.ListResources(auth, "/coupons", q, returnAll)
	if err != nil {
		return woocommerce.ErrorResult(err.Error()), nil
	}
	out := woocommerce.ListResult(items, total, totalPages, "")
	if returnAll && pages >= woocommerce.MaxAllPages && totalPages > pages {
		out["tool_result"] = fmt.Sprintf("Fetched %d coupon(s) across %d page(s); stopped at the %d-page safety cap — narrow the filters or page through manually to get the rest", len(items), pages, woocommerce.MaxAllPages)
	} else if returnAll {
		out["tool_result"] = fmt.Sprintf("Fetched all %d coupon(s) across %d page(s)", len(items), pages)
	} else {
		out["tool_result"] = fmt.Sprintf("Found %d coupon(s)", len(items))
	}
	return out, nil
}
