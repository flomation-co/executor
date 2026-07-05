package cms_wordpress_page_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Create Page"
	Description  = "Create a page on your WordPress site. Set common fields directly (title, content, status, parent, menu order) or add any other page field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "wordpress+plus"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-site.com — your WordPress site root, not the /wp-json path", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "Your WordPress username", Required: true},
	{Name: "app_password", Type: core.ConnectionTypeSecret, Label: "Application Password", Placeholder: "Users ▸ Profile ▸ Application Passwords (WP 5.6+)", Required: true},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure SSL", Placeholder: "Skip TLS verification — only for self-signed sites"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Content", Placeholder: "HTML or block markup"},
	{Name: "excerpt", Type: core.ConnectionTypeText, Label: "Excerpt"},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
		Options: []core.ConnectionOption{
			{Name: "Publish", Value: "publish"},
			{Name: "Draft", Value: "draft"},
			{Name: "Pending Review", Value: "pending"},
			{Name: "Private", Value: "private"},
			{Name: "Scheduled (future)", Value: "future"},
		},
	},
	{Name: "author", Type: core.ConnectionTypeString, Label: "Author (User ID)", Placeholder: "Author user ID (leave blank for the authenticating user)"},
	{Name: "slug", Type: core.ConnectionTypeString, Label: "Slug"},
	{Name: "parent", Type: core.ConnectionTypeString, Label: "Parent Page ID", Placeholder: "ID of the parent page to nest this under"},
	{Name: "featured_media", Type: core.ConnectionTypeString, Label: "Featured Media (ID)", Placeholder: "Media/attachment ID for the featured image"},
	{Name: "menu_order", Type: core.ConnectionTypeString, Label: "Menu Order", Placeholder: "Sort position among sibling pages, e.g. 0"},
	{Name: "password", Type: core.ConnectionTypeString, Label: "Page Password", Placeholder: "Protect the page with a password (distinct from the Application Password above)"},
	{
		Name:  "comment_status",
		Type:  core.ConnectionTypeString,
		Label: "Comments",
		Options: []core.ConnectionOption{
			{Name: "Open", Value: "open"},
			{Name: "Closed", Value: "closed"},
		},
	},
	{
		Name:  "ping_status",
		Type:  core.ConnectionTypeString,
		Label: "Pingbacks",
		Options: []core.ConnectionOption{
			{Name: "Open", Value: "open"},
			{Name: "Closed", Value: "closed"},
		},
	},
	{Name: "date", Type: core.ConnectionTypeDateTime, Label: "Date", Placeholder: "Publish date (defaults to now)"},
	{Name: "template", Type: core.ConnectionTypeString, Label: "Template"},
	{Name: "meta", Type: core.ConnectionTypeObject, Label: "Meta (JSON)", Placeholder: `{"my_key":"my_value"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"date_gmt":"2026-01-01T09:00:00"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Page ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Page"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := wordpress.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	title, err := wordpress.RequiredString("title", inputs)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	page := map[string]interface{}{"title": title}
	wordpress.SetIfPresent(page, inputs, "content", "content")
	wordpress.SetIfPresent(page, inputs, "excerpt", "excerpt")
	wordpress.SetIfPresent(page, inputs, "status", "status")
	wordpress.SetIfPresent(page, inputs, "slug", "slug")
	wordpress.SetIfPresent(page, inputs, "comment_status", "comment_status")
	wordpress.SetIfPresent(page, inputs, "ping_status", "ping_status")
	wordpress.SetIfPresent(page, inputs, "date", "date")
	wordpress.SetIfPresent(page, inputs, "template", "template")
	wordpress.SetIfPresent(page, inputs, "password", "password")
	for _, field := range []string{"author", "parent", "featured_media", "menu_order"} {
		if err := wordpress.SetIntIfPresent(page, inputs, field, field); err != nil {
			return wordpress.ErrorResult(err.Error()), nil
		}
	}
	if err := wordpress.SetJSONIfPresent(page, inputs, "meta", "meta"); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	if err := wordpress.MergeAdditionalFields(page, inputs); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	resp, err := wordpress.CreateResource(auth, "/pages", page)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ResourceResult(resp, "")
	out["tool_result"] = fmt.Sprintf("Created page %s", out["id"])
	return out, nil
}
