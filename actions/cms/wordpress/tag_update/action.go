package cms_wordpress_tag_update

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Update Tag"
	Description  = "Update an existing tag on your WordPress site. Only the fields you set are changed; add any other tag field via Additional Fields."
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
	{Name: "tag_id", Type: core.ConnectionTypeString, Label: "Tag ID", Placeholder: "123", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description"},
	{Name: "slug", Type: core.ConnectionTypeString, Label: "Slug"},
	{Name: "meta", Type: core.ConnectionTypeObject, Label: "Meta (JSON)", Placeholder: `{"my_key":"my_value"}`},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Tag ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Tag"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := wordpress.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	tagID, err := wordpress.RequiredString("tag_id", inputs)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	// Tags are a flat taxonomy — no parent field, unlike categories.
	tag := map[string]interface{}{}
	wordpress.SetIfPresent(tag, inputs, "name", "name")
	wordpress.SetIfPresent(tag, inputs, "description", "description")
	wordpress.SetIfPresent(tag, inputs, "slug", "slug")
	if err := wordpress.SetJSONIfPresent(tag, inputs, "meta", "meta"); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	if err := wordpress.MergeAdditionalFields(tag, inputs); err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}

	resp, err := wordpress.UpdateResource(auth, "/tags/"+url.PathEscape(tagID), tag)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ResourceResult(resp, fmt.Sprintf("Updated tag %s", tagID))
	return out, nil
}
