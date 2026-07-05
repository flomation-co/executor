package cms_wordpress_user_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Update User"
	Description  = "Update an existing user on your WordPress site. Only the fields you set are changed; add any other user field via Additional Fields."
	Website      = "https://www.flomation.co"
	Icon         = "wordpress+pencil"
	Date         = "05/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "url", Type: core.ConnectionTypeString, Label: "Site URL", Placeholder: "https://your-site.com — your WordPress site root, not the /wp-json path", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "Your WordPress username", Required: true},
	{Name: "app_password", Type: core.ConnectionTypeSecret, Label: "Application Password", Placeholder: "Users ▸ Profile ▸ Application Passwords (WP 5.6+)", Required: true},
	{Name: "allow_insecure", Type: core.ConnectionTypeBoolean, Label: "Allow Insecure SSL", Placeholder: "Skip TLS verification — only for self-signed sites"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID", Placeholder: "123", Required: true},
	// WordPress usernames are immutable, so there is no username field here.
	{Name: "name", Type: core.ConnectionTypeString, Label: "Display Name"},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "user@example.com"},
	// user_url (not url) avoids colliding with the auth Site URL input above; it
	// maps to WordPress's "url" user field (the user's website).
	{Name: "user_url", Type: core.ConnectionTypeString, Label: "Website URL", Placeholder: "https://the-user-website.com"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Biographical info shown on the profile"},
	{Name: "nickname", Type: core.ConnectionTypeString, Label: "Nickname"},
	{Name: "slug", Type: core.ConnectionTypeString, Label: "Slug"},
	// The account password — named "password" (the auth credential is
	// app_password), so it can use the natural field name without colliding.
	{Name: "password", Type: core.ConnectionTypeString, Label: "Password", Placeholder: "Set a new account password (leave blank to keep the current one)"},
	{Name: "roles", Type: core.ConnectionTypeString, Label: "Roles", Placeholder: "Comma-separated role slugs, e.g. author,editor (replaces the set)"},
	{Name: "meta", Type: core.ConnectionTypeObject, Label: "Meta (JSON)", Placeholder: `{"my_key":"my_value"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)"},
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

	user := map[string]interface{}{}
	wordpress.SetIfPresent(user, inputs, "name", "name")
	wordpress.SetIfPresent(user, inputs, "first_name", "first_name")
	wordpress.SetIfPresent(user, inputs, "last_name", "last_name")
	wordpress.SetIfPresent(user, inputs, "email", "email")
	wordpress.SetIfPresent(user, inputs, "url", "user_url")
	wordpress.SetIfPresent(user, inputs, "description", "description")
	wordpress.SetIfPresent(user, inputs, "nickname", "nickname")
	wordpress.SetIfPresent(user, inputs, "slug", "slug")
	wordpress.SetIfPresent(user, inputs, "password", "password")
	wordpress.SetStringListIfPresent(user, inputs, "roles", "roles")
	if err := wordpress.SetJSONIfPresent(user, inputs, "meta", "meta"); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	if err := wordpress.MergeAdditionalFields(user, inputs); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	resp, err := wordpress.UpdateResource(auth, "/users/"+url.PathEscape(userID), user)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ResourceResult(resp, fmt.Sprintf("Updated user %s", userID))
	return out, nil
}
