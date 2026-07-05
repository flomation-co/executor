package cms_wordpress_comment_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Create Comment"
	Description  = "Create a comment on a post on your WordPress site. Set the author and body directly, or add any other comment field via Additional Fields."
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
	{Name: "post", Type: core.ConnectionTypeString, Label: "Post ID", Placeholder: "ID of the post to comment on", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Content", Placeholder: "The comment body", Required: true},
	{Name: "author", Type: core.ConnectionTypeString, Label: "Author (User ID)", Placeholder: "User ID of the comment author (leave blank for the authenticating user)"},
	{Name: "author_name", Type: core.ConnectionTypeString, Label: "Author Name", Placeholder: "Display name for an anonymous author"},
	{Name: "author_email", Type: core.ConnectionTypeString, Label: "Author Email"},
	{Name: "author_url", Type: core.ConnectionTypeString, Label: "Author URL"},
	{Name: "parent", Type: core.ConnectionTypeString, Label: "Parent Comment ID", Placeholder: "Reply to this comment ID (leave blank for a top-level comment)"},
	{Name: "date", Type: core.ConnectionTypeDateTime, Label: "Date", Placeholder: "Comment date (defaults to now)"},
	{
		Name:  "status",
		Type:  core.ConnectionTypeString,
		Label: "Status",
		Options: []core.ConnectionOption{
			{Name: "Approved", Value: "approved"},
			{Name: "Hold", Value: "hold"},
			{Name: "Spam", Value: "spam"},
		},
	},
	{Name: "meta", Type: core.ConnectionTypeObject, Label: "Meta (JSON)", Placeholder: `{"my_key":"my_value"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)", Placeholder: `{"author_ip":"127.0.0.1"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Comment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Comment"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := wordpress.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	if _, err := wordpress.RequiredString("post", inputs); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	if _, err := wordpress.RequiredString("content", inputs); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	comment := map[string]interface{}{}
	for _, field := range []string{"post", "author", "parent"} {
		if err := wordpress.SetIntIfPresent(comment, inputs, field, field); err != nil {
			return wordpress.ErrorResult(err.Error()), nil
		}
	}
	wordpress.SetIfPresent(comment, inputs, "content", "content")
	wordpress.SetIfPresent(comment, inputs, "author_name", "author_name")
	wordpress.SetIfPresent(comment, inputs, "author_email", "author_email")
	wordpress.SetIfPresent(comment, inputs, "author_url", "author_url")
	wordpress.SetIfPresent(comment, inputs, "date", "date")
	wordpress.SetIfPresent(comment, inputs, "status", "status")
	if err := wordpress.SetJSONIfPresent(comment, inputs, "meta", "meta"); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	if err := wordpress.MergeAdditionalFields(comment, inputs); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	resp, err := wordpress.CreateResource(auth, "/comments", comment)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ResourceResult(resp, "")
	out["tool_result"] = fmt.Sprintf("Created comment %s", out["id"])
	return out, nil
}
