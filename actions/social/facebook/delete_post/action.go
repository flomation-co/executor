package facebook_delete_post

import (
	"fmt"

	core "flomation.app/automate/executor"
	fb "flomation.app/automate/executor/actions/social/facebook"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Facebook Delete Post"
	Description  = "Delete a Facebook Page post by ID"
	Website      = "https://www.flomation.co"
	Icon         = "facebook+trash"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "Page Access Token", Placeholder: "${first_page_token}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeString, Label: "App Secret", Placeholder: "${secrets.facebook_app_secret}"},
	{Name: "post_id", Type: core.ConnectionTypeString, Label: "Post ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := fb.GetAccessToken(inputs)
	if err != nil {
		return fb.ErrorResult(err.Error()), nil
	}

	postID := fb.OptionalString("post_id", inputs)
	if postID == "" {
		return fb.ErrorResult("post_id is required"), nil
	}

	appSecret := fb.GetAppSecret(inputs)
	endpoint := fmt.Sprintf("%s/%s", fb.GraphAPIBase, postID)
	resp, err := fb.ExecuteAPI(token, appSecret, "DELETE", endpoint, nil)
	if err != nil {
		return fb.ErrorResult(fmt.Sprintf("failed to delete post: %v", err)), nil
	}

	if err := fb.CheckResponse(resp); err != nil {
		return fb.ErrorResult(err.Error()), nil
	}

	return fb.SuccessResult("Post deleted successfully", nil), nil
}
