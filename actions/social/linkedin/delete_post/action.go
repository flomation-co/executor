package linkedin_delete_post

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	linkedin "flomation.app/automate/executor/actions/social/linkedin"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "LinkedIn Delete Post"
	Description  = "Delete a LinkedIn post by its URN"
	Website      = "https://www.flomation.co"
	Icon         = "linkedin+trash"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "LinkedIn Access Token", Placeholder: "${credentials.linkedin}", Required: true},
	{Name: "post_urn", Type: core.ConnectionTypeString, Label: "Post URN", Placeholder: "urn:li:ugcPost:...", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := linkedin.GetAccessToken(inputs)
	if err != nil {
		return linkedin.ErrorResult(err.Error()), nil
	}

	postURN := linkedin.OptionalString("post_urn", inputs)
	if postURN == "" {
		return linkedin.ErrorResult("post_urn is required"), nil
	}

	encodedURN := url.PathEscape(postURN)
	resp, err := linkedin.ExecuteAPI(token, "DELETE", linkedin.BaseURL+"/ugcPosts/"+encodedURN, nil)
	if err != nil {
		return linkedin.ErrorResult(fmt.Sprintf("failed to delete post: %v", err)), nil
	}

	if err := linkedin.CheckResponse(resp); err != nil {
		return linkedin.ErrorResult(err.Error()), nil
	}

	return linkedin.SuccessResult("Post deleted successfully", nil), nil
}
