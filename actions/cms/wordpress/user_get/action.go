package cms_wordpress_user_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Get User"
	Description  = "Retrieve a single user from your WordPress site by ID."
	Website      = "https://www.flomation.co"
	Icon         = "wordpress+eye"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-site.com — your WordPress site root, not the /wp-json path", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "Your WordPress username", Required: true},
	{Name: "app_password", Type: core.ConnectionTypeSecret, Label: "Application Password", Placeholder: "Users ▸ Profile ▸ Application Passwords (WP 5.6+)", Required: true},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure SSL", Placeholder: "Skip TLS verification — only for self-signed sites"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID", Placeholder: "123", Required: true},
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
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "User"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := wordpress.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	userID, err := wordpress.RequiredString("user_id", inputs)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	wordpress.AddFilter(q, inputs, "context", "context")

	resp, err := wordpress.GetResource(auth, "/users/"+url.PathEscape(userID), q)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ResourceResult(resp, fmt.Sprintf("Retrieved user %s", userID))
	return out, nil
}
