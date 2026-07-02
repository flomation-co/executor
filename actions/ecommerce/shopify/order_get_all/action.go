package ecommerce_shopify_order_get_all

import (
	"fmt"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	shopify "flomation.app/automate/executor/actions/ecommerce/shopify"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Shopify: Get Many Orders"
	Description  = "List orders from your Shopify store, with optional filters. Enable Return All to auto-paginate every matching order."
	Website      = "https://www.flomation.co"
	Icon         = "shopify+list"
	Date         = "02/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "shop", Type: core.ConnectionTypeString, Label: "Shop Subdomain", Placeholder: "my-store (from my-store.myshopify.com)", Required: true},
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Admin API Access Token", Placeholder: "shpat_... — or use Client ID + Secret below"},
	{Name: "client_id", Type: core.ConnectionTypeString, Label: "Client ID", Placeholder: "Dev Dashboard app Client ID (if not using a token)"},
	{Name: "client_secret", Type: core.ConnectionTypeSecret, Label: "Client Secret", Placeholder: "shpss_... — minted into a 24h token automatically"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (auto-paginate every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "50 per page (max 250); Return All still fetches every match"},
	{Name: "page_info", Type: core.ConnectionTypeString, Label: "Page Info (cursor)", Placeholder: "Next-page cursor from a previous run (ignored when Return All is on)"},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
		Options: []core.ConnectionOption{
			{Name: "Open", Value: "open"},
			{Name: "Closed", Value: "closed"},
			{Name: "Cancelled", Value: "cancelled"},
			{Name: "Any", Value: "any"},
		},
	},
	{
		Name:  "financial_status",
		Type:  core.ConnectionTypeString,
		Label: "Financial Status",
		Options: []core.ConnectionOption{
			{Name: "Any", Value: "any"},
			{Name: "Authorized", Value: "authorized"},
			{Name: "Paid", Value: "paid"},
			{Name: "Partially Paid", Value: "partially_paid"},
			{Name: "Partially Refunded", Value: "partially_refunded"},
			{Name: "Pending", Value: "pending"},
			{Name: "Refunded", Value: "refunded"},
			{Name: "Unpaid", Value: "unpaid"},
			{Name: "Voided", Value: "voided"},
		},
	},
	{
		Name:  "fulfillment_status",
		Type:  core.ConnectionTypeString,
		Label: "Fulfillment Status",
		Options: []core.ConnectionOption{
			{Name: "Any", Value: "any"},
			{Name: "Shipped", Value: "shipped"},
			{Name: "Partial", Value: "partial"},
			{Name: "Unshipped", Value: "unshipped"},
			{Name: "Unfulfilled", Value: "unfulfilled"},
		},
	},
	{Name: "ids", Type: core.ConnectionTypeString, Label: "IDs", Placeholder: "Comma-separated order IDs"},
	{Name: "since_id", Type: core.ConnectionTypeString, Label: "Since ID", Placeholder: "Return orders after this ID"},
	{Name: "created_at_min", Type: core.ConnectionTypeDateTime, Label: "Created After"},
	{Name: "created_at_max", Type: core.ConnectionTypeDateTime, Label: "Created Before"},
	{Name: "updated_at_min", Type: core.ConnectionTypeDateTime, Label: "Updated After"},
	{Name: "updated_at_max", Type: core.ConnectionTypeDateTime, Label: "Updated Before"},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Orders"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "next_page_info", Type: core.ConnectionTypeString, Label: "Next Page Info"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	shop, token, err := shopify.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	returnAll := shopify.OptionalBool("return_all", inputs)
	q := url.Values{}

	if pageInfo := shopify.OptionalString("page_info", inputs); pageInfo != "" && !returnAll {
		// A resume cursor was supplied: Shopify permits only limit + fields
		// alongside page_info, so ignore the filter fields this page.
		q.Set("page_info", pageInfo)
		if limit, set := shopify.OptionalInt("limit", inputs); set {
			q.Set("limit", strconv.Itoa(shopify.ClampLimit(limit, set)))
		}
		shopify.AddFilter(q, inputs, "fields", "fields")
	} else {
		limit, set := shopify.OptionalInt("limit", inputs)
		q.Set("limit", strconv.Itoa(shopify.ClampLimit(limit, set)))
		shopify.AddFilter(q, inputs, "status", "status")
		shopify.AddFilter(q, inputs, "financial_status", "financial_status")
		shopify.AddFilter(q, inputs, "fulfillment_status", "fulfillment_status")
		shopify.AddFilter(q, inputs, "ids", "ids")
		shopify.AddFilter(q, inputs, "since_id", "since_id")
		shopify.AddFilter(q, inputs, "created_at_min", "created_at_min")
		shopify.AddFilter(q, inputs, "created_at_max", "created_at_max")
		shopify.AddFilter(q, inputs, "updated_at_min", "updated_at_min")
		shopify.AddFilter(q, inputs, "updated_at_max", "updated_at_max")
		shopify.AddFilter(q, inputs, "fields", "fields")
	}

	items, next, lastRaw, pages, err := shopify.ListResources(shop, token, "/orders.json", "orders", q, returnAll)
	if err != nil {
		return shopify.ErrorResult(err.Error()), nil
	}
	out := shopify.ListResult(items, next, lastRaw, "")
	if returnAll && next != "" && pages >= shopify.MaxAllPages {
		out["tool_result"] = fmt.Sprintf("Fetched %d order(s) across %d page(s); stopped at the %d-page safety cap — pass the returned page info to continue", len(items), pages, shopify.MaxAllPages)
	} else if returnAll {
		out["tool_result"] = fmt.Sprintf("Fetched all %d order(s) across %d page(s)", len(items), pages)
	} else {
		out["tool_result"] = fmt.Sprintf("Found %d order(s)", len(items))
	}
	return out, nil
}
