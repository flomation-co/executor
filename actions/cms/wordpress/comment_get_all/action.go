package cms_wordpress_comment_get_all

import (
	"fmt"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Get Many Comments"
	Description  = "List comments from your WordPress site, with optional filters. Enable Return All to auto-paginate every matching comment."
	Website      = "https://www.flomation.co"
	Icon         = "wordpress+list"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-site.com — your WordPress site root, not the /wp-json path", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "Your WordPress username", Required: true},
	{Name: "app_password", Type: core.ConnectionTypeSecret, Label: "Application Password", Placeholder: "Users ▸ Profile ▸ Application Passwords (WP 5.6+)", Required: true},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure SSL", Placeholder: "Skip TLS verification — only for self-signed sites"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (auto-paginate every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit (per page)", Placeholder: "50 per page (max 100); Return All still fetches every match"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Page number to fetch (ignored when Return All is on)"},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search"},
	{Name: "post", Type: core.ConnectionTypeString, Label: "Post ID", Placeholder: "Limit to comments on this post ID"},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
		Options: []core.ConnectionOption{
			{Name: "Approved", Value: "approved"},
			{Name: "Hold", Value: "hold"},
			{Name: "Spam", Value: "spam"},
			{Name: "Trash", Value: "trash"},
		},
	},
	{Name: "author_email", Type: core.ConnectionTypeString, Label: "Author Email"},
	{Name: "parent", Type: core.ConnectionTypeString, Label: "Parent Comment ID", Placeholder: "Limit to replies to this comment ID"},
	{Name: "after", Type: core.ConnectionTypeDateTime, Label: "Published After"},
	{Name: "before", Type: core.ConnectionTypeDateTime, Label: "Published Before"},
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
			{Name: "Date GMT", Value: "date_gmt"},
			{Name: "ID", Value: "id"},
			{Name: "Post", Value: "post"},
			{Name: "Parent", Value: "parent"},
			{Name: "Type", Value: "type"},
		},
	},
	{Name: "include", Type: core.ConnectionTypeString, Label: "Include IDs", Placeholder: "Comma-separated comment IDs to include"},
	{Name: "exclude", Type: core.ConnectionTypeString, Label: "Exclude IDs", Placeholder: "Comma-separated comment IDs to exclude"},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset", Placeholder: "Skip this many results (ignored when Return All is on)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Comments"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count (this response)"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total (site-wide match)"},
	{Name: "total_pages", Type: core.ConnectionTypeInteger, Label: "Total Pages"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := wordpress.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	returnAll := wordpress.OptionalBool("return_all", inputs)
	q := url.Values{}
	limit, set := wordpress.OptionalInt("limit", inputs)
	q.Set("per_page", strconv.Itoa(wordpress.ClampLimit(limit, set)))
	if !returnAll {
		if page, ok := wordpress.OptionalInt("page", inputs); ok && page > 0 {
			q.Set("page", strconv.Itoa(page))
		}
		if offset, ok := wordpress.OptionalInt("offset", inputs); ok {
			q.Set("offset", strconv.Itoa(offset))
		}
	}
	wordpress.AddFilter(q, inputs, "search", "search")
	wordpress.AddFilter(q, inputs, "post", "post")
	wordpress.AddFilter(q, inputs, "status", "status")
	wordpress.AddFilter(q, inputs, "author_email", "author_email")
	wordpress.AddFilter(q, inputs, "parent", "parent")
	wordpress.AddFilter(q, inputs, "after", "after")
	wordpress.AddFilter(q, inputs, "before", "before")
	wordpress.AddFilter(q, inputs, "order", "order")
	wordpress.AddFilter(q, inputs, "orderby", "orderby")
	wordpress.AddFilter(q, inputs, "include", "include")
	wordpress.AddFilter(q, inputs, "exclude", "exclude")

	items, total, totalPages, pages, err := wordpress.ListResources(auth, "/comments", q, returnAll)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ListResult(items, total, totalPages, "")
	if returnAll && pages >= wordpress.MaxAllPages && totalPages > pages {
		out["tool_result"] = fmt.Sprintf("Fetched %d comment(s) across %d page(s); stopped at the %d-page safety cap — narrow the filters or page through manually to get the rest", len(items), pages, wordpress.MaxAllPages)
	} else if returnAll {
		out["tool_result"] = fmt.Sprintf("Fetched all %d comment(s) across %d page(s)", len(items), pages)
	} else {
		out["tool_result"] = fmt.Sprintf("Found %d comment(s)", len(items))
	}
	return out, nil
}
