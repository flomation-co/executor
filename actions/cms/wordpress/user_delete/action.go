package cms_wordpress_user_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Delete User"
	Description  = "Delete a user from your WordPress site, reassigning their content to another user (required by WordPress)."
	Website      = "https://www.flomation.co"
	Icon         = "wordpress+trash"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-site.com — your WordPress site root, not the /wp-json path", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "Your WordPress username", Required: true},
	{Name: "app_password", Type: core.ConnectionTypeSecret, Label: "Application Password", Placeholder: "Users ▸ Profile ▸ Application Passwords (WP 5.6+)", Required: true},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure SSL", Placeholder: "Skip TLS verification — only for self-signed sites"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID", Placeholder: "123", Required: true},
	{Name: "reassign", Type: core.ConnectionTypeString, Label: "Reassign Content To (User ID)", Placeholder: "User ID to inherit this user's posts — WordPress requires this", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deleted User"},
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
	// WordPress users have no trash, so a plain DELETE is rejected: deleting a
	// user ALWAYS requires force=true AND a reassign target (the user ID that
	// inherits this user's content). Both are mandatory here — there is no
	// trash/force toggle, and reassign must be supplied.
	reassign, err := wordpress.RequiredString("reassign", inputs)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	resp, err := wordpress.DeleteResource(auth, "/users/"+url.PathEscape(userID), true, url.Values{"reassign": {reassign}})
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ResourceResult(resp, fmt.Sprintf("Deleted user %s", userID))
	return out, nil
}
