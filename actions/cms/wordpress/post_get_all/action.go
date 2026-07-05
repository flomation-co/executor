package cms_wordpress_post_get_all

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
	Name         = "WordPress: Get Many Posts"
	Description  = "List posts from your WordPress site, with optional filters. Enable Return All to auto-paginate every matching post."
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
	{
		Name:  "context",
		Type:  core.ConnectionTypeString,
		Label: "Context",
		Options: []core.ConnectionOption{
			{Name: "View", Value: "view"},
			{Name: "Edit (more fields; needs edit rights)", Value: "edit"},
			{Name: "Embed", Value: "embed"},
		},
	},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All (auto-paginate every match)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit (per page)", Placeholder: "50 per page (max 100); Return All still fetches every match"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "Page number to fetch (ignored when Return All is on)"},
	{Name: "search", Type: core.ConnectionTypeString, Label: "Search"},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
		Options: []core.ConnectionOption{
			{Name: "Publish", Value: "publish"},
			{Name: "Draft", Value: "draft"},
			{Name: "Pending Review", Value: "pending"},
			{Name: "Private", Value: "private"},
			{Name: "Future", Value: "future"},
			{Name: "Trash", Value: "trash"},
			{Name: "Any", Value: "any"},
		},
	},
	{Name: "author", Type: core.ConnectionTypeString, Label: "Author (User ID)", Placeholder: "Limit to posts by this author user ID"},
	{Name: "categories", Type: core.ConnectionTypeString, Label: "Category IDs", Placeholder: "Comma-separated category IDs"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tag IDs", Placeholder: "Comma-separated tag IDs"},
	{Name: "categories_exclude", Type: core.ConnectionTypeString, Label: "Exclude Category IDs", Placeholder: "Comma-separated category IDs to exclude"},
	{Name: "tags_exclude", Type: core.ConnectionTypeString, Label: "Exclude Tag IDs", Placeholder: "Comma-separated tag IDs to exclude"},
	{Name: "slug", Type: core.ConnectionTypeString, Label: "Slug"},
	{Name: "after", Type: core.ConnectionTypeDateTime, Label: "Published After"},
	{Name: "before", Type: core.ConnectionTypeDateTime, Label: "Published Before"},
	{Name: "include", Type: core.ConnectionTypeString, Label: "Include IDs", Placeholder: "Comma-separated post IDs to include"},
	{Name: "exclude", Type: core.ConnectionTypeString, Label: "Exclude IDs", Placeholder: "Comma-separated post IDs to exclude"},
	{Name: "sticky", Type: core.ConnectionTypeBoolean, Label: "Sticky Only"},
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
			{Name: "Modified", Value: "modified"},
			{Name: "ID", Value: "id"},
			{Name: "Title", Value: "title"},
			{Name: "Slug", Value: "slug"},
			{Name: "Author", Value: "author"},
			{Name: "Relevance", Value: "relevance"},
		},
	},
	{Name: "offset", Type: core.ConnectionTypeInteger, Label: "Offset", Placeholder: "Skip this many results (ignored when Return All is on)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Posts"},
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
	wordpress.AddFilter(q, inputs, "context", "context")
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
	wordpress.AddFilter(q, inputs, "status", "status")
	wordpress.AddFilter(q, inputs, "author", "author")
	wordpress.AddFilter(q, inputs, "categories", "categories")
	wordpress.AddFilter(q, inputs, "tags", "tags")
	wordpress.AddFilter(q, inputs, "categories_exclude", "categories_exclude")
	wordpress.AddFilter(q, inputs, "tags_exclude", "tags_exclude")
	wordpress.AddFilter(q, inputs, "slug", "slug")
	wordpress.AddFilter(q, inputs, "after", "after")
	wordpress.AddFilter(q, inputs, "before", "before")
	wordpress.AddFilter(q, inputs, "include", "include")
	wordpress.AddFilter(q, inputs, "exclude", "exclude")
	wordpress.AddFilter(q, inputs, "order", "order")
	wordpress.AddFilter(q, inputs, "orderby", "orderby")
	if wordpress.OptionalBool("sticky", inputs) {
		q.Set("sticky", "true")
	}

	items, total, totalPages, pages, err := wordpress.ListResources(auth, "/posts", q, returnAll)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ListResult(items, total, totalPages, "")
	if returnAll && pages >= wordpress.MaxAllPages && totalPages > pages {
		out["tool_result"] = fmt.Sprintf("Fetched %d post(s) across %d page(s); stopped at the %d-page safety cap — narrow the filters or page through manually to get the rest", len(items), pages, wordpress.MaxAllPages)
	} else if returnAll {
		out["tool_result"] = fmt.Sprintf("Fetched all %d post(s) across %d page(s)", len(items), pages)
	} else {
		out["tool_result"] = fmt.Sprintf("Found %d post(s)", len(items))
	}
	return out, nil
}
