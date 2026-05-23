package facebook_publish_post

import (
	"encoding/json"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	fb "flomation.app/automate/executor/actions/social/facebook"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Facebook Publish Post"
	Description  = "Publish a post to a Facebook Page"
	Website      = "https://www.flomation.co"
	Icon         = "facebook"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "Page Access Token", Placeholder: "${first_page_token}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeString, Label: "App Secret", Placeholder: "${secrets.facebook_app_secret}"},
	{Name: "page_id", Type: core.ConnectionTypeString, Label: "Page ID", Placeholder: "${first_page_id}", Required: true},
	{Name: "message", Type: core.ConnectionTypeText, Label: "Message", Required: true},
	{Name: "link", Type: core.ConnectionTypeString, Label: "Link URL", Placeholder: "https://example.com (optional)"},
	{Name: "published", Type: core.ConnectionTypeBoolean, Label: "Published", Placeholder: "true"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
	{Name: "post_id", Type: core.ConnectionTypeString, Label: "Post ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := fb.GetAccessToken(inputs)
	if err != nil {
		return fb.ErrorResult(err.Error()), nil
	}

	pageID := fb.OptionalString("page_id", inputs)
	if pageID == "" {
		return fb.ErrorResult("page_id is required — use Facebook Get Pages to obtain it"), nil
	}

	message := fb.OptionalString("message", inputs)
	if message == "" {
		return fb.ErrorResult("message is required"), nil
	}

	params := url.Values{
		"message": {message},
	}

	link := fb.OptionalString("link", inputs)
	if link != "" {
		params.Set("link", link)
	}

	publishedConn := core.FindConnection("published", inputs)
	if publishedConn != nil && publishedConn.Boolean() != nil && !*publishedConn.Boolean() {
		params.Set("published", "false")
	}

	appSecret := fb.GetAppSecret(inputs)
	endpoint := fmt.Sprintf("%s/%s/feed", fb.GraphAPIBase, pageID)
	resp, err := fb.ExecuteAPI(token, appSecret, "POST", endpoint, params)
	if err != nil {
		return fb.ErrorResult(fmt.Sprintf("failed to publish post: %v", err)), nil
	}

	if err := fb.CheckResponse(resp); err != nil {
		return fb.ErrorResult(err.Error()), nil
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return fb.ErrorResult(fmt.Sprintf("failed to parse response: %v", err)), nil
	}

	return fb.SuccessResult(
		fmt.Sprintf("Post published: %s", result.ID),
		map[string]interface{}{
			"post_id": result.ID,
		},
	), nil
}
