package cms_wordpress_post_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Update Post"
	Description  = "Update an existing post on your WordPress site. Only the fields you set are changed; add any other post field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "wordpress+pen"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-site.com — your WordPress site root, not the /wp-json path", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "Your WordPress username", Required: true},
	{Name: "app_password", Type: core.ConnectionTypeSecret, Label: "Application Password", Placeholder: "Users ▸ Profile ▸ Application Passwords (WP 5.6+)", Required: true},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure SSL", Placeholder: "Skip TLS verification — only for self-signed sites"},
	{Name: "post_id", Type: core.ConnectionTypeString, Label: "Post ID", Placeholder: "123", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
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
	{Name: "author", Type: core.ConnectionTypeString, Label: "Author (User ID)"},
	{Name: "slug", Type: core.ConnectionTypeString, Label: "Slug"},
	{Name: "categories", Type: core.ConnectionTypeString, Label: "Category IDs", Placeholder: "Comma-separated category IDs (replaces the set)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Tag IDs", Placeholder: "Comma-separated tag IDs (replaces the set)"},
	{
		Name:  "format",
		Type:  core.ConnectionTypeString,
		Label: "Format",
		Options: []core.ConnectionOption{
			{Name: "Standard", Value: "standard"},
			{Name: "Aside", Value: "aside"},
			{Name: "Gallery", Value: "gallery"},
			{Name: "Link", Value: "link"},
			{Name: "Image", Value: "image"},
			{Name: "Quote", Value: "quote"},
			{Name: "Status", Value: "status"},
			{Name: "Video", Value: "video"},
			{Name: "Audio", Value: "audio"},
			{Name: "Chat", Value: "chat"},
		},
	},
	{Name: "featured_media", Type: core.ConnectionTypeString, Label: "Featured Media (ID)"},
	{Name: "sticky", Type: core.ConnectionTypeBoolean, Label: "Sticky"},
	{Name: "password", Type: core.ConnectionTypeString, Label: "Post Password"},
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
	{Name: "date", Type: core.ConnectionTypeDateTime, Label: "Date"},
	{Name: "template", Type: core.ConnectionTypeString, Label: "Template"},
	{Name: "meta", Type: core.ConnectionTypeObject, Label: "Meta (JSON)", Placeholder: `{"my_key":"my_value"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Post ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Post"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := wordpress.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	postID, err := wordpress.RequiredString("post_id", inputs)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	post := map[string]interface{}{}
	wordpress.SetIfPresent(post, inputs, "title", "title")
	wordpress.SetIfPresent(post, inputs, "content", "content")
	wordpress.SetIfPresent(post, inputs, "excerpt", "excerpt")
	wordpress.SetIfPresent(post, inputs, "status", "status")
	wordpress.SetIfPresent(post, inputs, "slug", "slug")
	wordpress.SetIfPresent(post, inputs, "format", "format")
	wordpress.SetIfPresent(post, inputs, "comment_status", "comment_status")
	wordpress.SetIfPresent(post, inputs, "ping_status", "ping_status")
	wordpress.SetIfPresent(post, inputs, "date", "date")
	wordpress.SetIfPresent(post, inputs, "template", "template")
	wordpress.SetIfPresent(post, inputs, "password", "password")
	wordpress.SetBoolIfSet(post, inputs, "sticky", "sticky")
	for _, field := range []string{"author", "featured_media"} {
		if err := wordpress.SetIntIfPresent(post, inputs, field, field); err != nil {
			return wordpress.ErrorResult(err.Error()), nil
		}
	}
	wordpress.SetIntListIfPresent(post, inputs, "categories", "categories")
	wordpress.SetIntListIfPresent(post, inputs, "tags", "tags")
	if err := wordpress.SetJSONIfPresent(post, inputs, "meta", "meta"); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	if err := wordpress.MergeAdditionalFields(post, inputs); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	resp, err := wordpress.UpdateResource(auth, "/posts/"+url.PathEscape(postID), post)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ResourceResult(resp, fmt.Sprintf("Updated post %s", postID))
	return out, nil
}
