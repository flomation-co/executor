package cms_wordpress_category_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Delete Category"
	Description  = "Delete a category from your WordPress site. Taxonomy terms cannot be trashed, so this always deletes the category permanently."
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
	{Name: "category_id", Type: core.ConnectionTypeString, Label: "Category ID", Placeholder: "123", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Category ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deleted Category"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := wordpress.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	categoryID, err := wordpress.RequiredString("category_id", inputs)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	// Taxonomy terms have no Trash, so WordPress rejects a plain DELETE and
	// requires force=true to remove a category — hence there is no trash/force
	// toggle here; a delete is always permanent.
	resp, err := wordpress.DeleteResource(auth, "/categories/"+url.PathEscape(categoryID), true, nil)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	out := wordpress.ResourceResult(resp, fmt.Sprintf("Deleted category %s", categoryID))
	return out, nil
}
