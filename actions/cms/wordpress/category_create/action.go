package cms_wordpress_category_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Create Category"
	Description  = "Create a category (a hierarchical taxonomy term) on your WordPress site. Set the name, description, slug and parent, or add any other field via Additional Fields."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description"},
	{Name: "slug", Type: core.ConnectionTypeString, Label: "Slug"},
	{Name: "parent", Type: core.ConnectionTypeString, Label: "Parent Category ID", Placeholder: "Category ID of the parent (leave blank for a top-level category)"},
	{Name: "meta", Type: core.ConnectionTypeObject, Label: "Meta (JSON)", Placeholder: `{"my_key":"my_value"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Category ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Category"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := wordpress.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	if _, err := wordpress.RequiredString("name", inputs); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	category := map[string]interface{}{}
	wordpress.SetIfPresent(category, inputs, "name", "name")
	wordpress.SetIfPresent(category, inputs, "description", "description")
	wordpress.SetIfPresent(category, inputs, "slug", "slug")
	if err := wordpress.SetIntIfPresent(category, inputs, "parent", "parent"); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	if err := wordpress.SetJSONIfPresent(category, inputs, "meta", "meta"); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	if err := wordpress.MergeAdditionalFields(category, inputs); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	resp, err := wordpress.CreateResource(auth, "/categories", category)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ResourceResult(resp, "")
	out["tool_result"] = fmt.Sprintf("Created category %s", out["id"])
	return out, nil
}
