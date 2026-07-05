package cms_wordpress_post_delete

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	wordpress "flomation.app/automate/executor/actions/cms/wordpress"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "WordPress: Delete Post"
	Description  = "Delete a post from your WordPress site. Moves it to the Trash by default; enable Permanently Delete to remove it for good."
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
	{Name: "post_id", Type: core.ConnectionTypeString, Label: "Post ID", Placeholder: "123", Required: true},
	{Name: "force", Type: core.ConnectionTypeBoolean, Label: "Permanently Delete", Placeholder: "On: delete permanently. Off (default): move to Trash."},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Post ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deleted Post"},
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
	// WordPress trashes a post on a plain DELETE; force=true removes it
	// permanently. Default off (trash) — the safer, WP-native behaviour.
	force := wordpress.OptionalBool("force", inputs)

	resp, err := wordpress.DeleteResource(auth, "/posts/"+url.PathEscape(postID), force, nil)
	if err != nil {
		return wordpress.ErrorResult(err.Error()), nil
	}
	verb := "Trashed"
	if force {
		verb = "Deleted"
	}
	out := wordpress.ResourceResult(resp, fmt.Sprintf("%s post %s", verb, postID))
	return out, nil
}
