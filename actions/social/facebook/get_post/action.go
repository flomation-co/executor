package facebook_get_post

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
	Name         = "Facebook Get Post"
	Description  = "Retrieve a Facebook post by ID with engagement metrics"
	Website      = "https://www.flomation.co"
	Icon         = "facebook+eye"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Page Access Token", Placeholder: "${first_page_token}", Required: true},
	{Name: "app_secret", Type: core.ConnectionTypeSecret, Label: "App Secret", Placeholder: "${secrets.facebook_app_secret}"},
	{Name: "post_id", Type: core.ConnectionTypeString, Label: "Post ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
	{Name: "message", Type: core.ConnectionTypeString, Label: "Message"},
	{Name: "created_time", Type: core.ConnectionTypeString, Label: "Created Time"},
	{Name: "likes", Type: core.ConnectionTypeInteger, Label: "Likes"},
	{Name: "comments", Type: core.ConnectionTypeInteger, Label: "Comments"},
	{Name: "shares", Type: core.ConnectionTypeInteger, Label: "Shares"},
	{Name: "post_json", Type: core.ConnectionTypeString, Label: "Full Post JSON"},
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

	params := url.Values{
		"fields": {"message,created_time,likes.summary(true),comments.summary(true),shares"},
	}

	appSecret := fb.GetAppSecret(inputs)
	endpoint := fmt.Sprintf("%s/%s", fb.GraphAPIBase, postID)
	resp, err := fb.ExecuteAPI(token, appSecret, "GET", endpoint, params)
	if err != nil {
		return fb.ErrorResult(fmt.Sprintf("failed to get post: %v", err)), nil
	}

	if err := fb.CheckResponse(resp); err != nil {
		return fb.ErrorResult(err.Error()), nil
	}

	var post map[string]interface{}
	if err := json.Unmarshal(resp.Body, &post); err != nil {
		return fb.ErrorResult(fmt.Sprintf("failed to parse post: %v", err)), nil
	}

	message, _ := post["message"].(string)
	createdTime, _ := post["created_time"].(string)

	var likes, comments, shares int64
	if likesData, ok := post["likes"].(map[string]interface{}); ok {
		if summary, ok := likesData["summary"].(map[string]interface{}); ok {
			if count, ok := summary["total_count"].(float64); ok {
				likes = int64(count)
			}
		}
	}
	if commentsData, ok := post["comments"].(map[string]interface{}); ok {
		if summary, ok := commentsData["summary"].(map[string]interface{}); ok {
			if count, ok := summary["total_count"].(float64); ok {
				comments = int64(count)
			}
		}
	}
	if sharesData, ok := post["shares"].(map[string]interface{}); ok {
		if count, ok := sharesData["count"].(float64); ok {
			shares = int64(count)
		}
	}

	postJSON, _ := json.Marshal(post)

	return fb.SuccessResult(
		fmt.Sprintf("Post: %s (Likes: %d, Comments: %d, Shares: %d)", truncate(message, 60), likes, comments, shares),
		map[string]interface{}{
			"message":      message,
			"created_time": createdTime,
			"likes":        likes,
			"comments":     comments,
			"shares":       shares,
			"post_json":    string(postJSON),
		},
	), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
