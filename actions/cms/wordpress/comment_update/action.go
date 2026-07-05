package cms_wordpress_comment_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Update Comment"
	Description  = "Update an existing comment on your WordPress site. Only the fields you set are changed; add any other comment field via Additional Fields."
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
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID", Placeholder: "123", Required: true},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Content", Placeholder: "The comment body"},
	{Name: "author_name", Type: core.ConnectionTypeString, Label: "Author Name"},
	{Name: "author_email", Type: core.ConnectionTypeString, Label: "Author Email"},
	{Name: "author_url", Type: core.ConnectionTypeString, Label: "Author URL"},
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
	{Name: "date", Type: core.ConnectionTypeDateTime, Label: "Date"},
	{Name: "meta", Type: core.ConnectionTypeObject, Label: "Meta (JSON)", Placeholder: `{"my_key":"my_value"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)"},
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
	commentID, err := wordpress.RequiredString("comment_id", inputs)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	comment := map[string]interface{}{}
	wordpress.SetIfPresent(comment, inputs, "content", "content")
	wordpress.SetIfPresent(comment, inputs, "author_name", "author_name")
	wordpress.SetIfPresent(comment, inputs, "author_email", "author_email")
	wordpress.SetIfPresent(comment, inputs, "author_url", "author_url")
	wordpress.SetIfPresent(comment, inputs, "status", "status")
	wordpress.SetIfPresent(comment, inputs, "date", "date")
	if err := wordpress.SetJSONIfPresent(comment, inputs, "meta", "meta"); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	if err := wordpress.MergeAdditionalFields(comment, inputs); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	resp, err := wordpress.UpdateResource(auth, "/comments/"+url.PathEscape(commentID), comment)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ResourceResult(resp, fmt.Sprintf("Updated comment %s", commentID))
	return out, nil
}
