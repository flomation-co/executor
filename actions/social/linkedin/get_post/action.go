package linkedin_get_post

import (
	"encoding/json"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	linkedin "flomation.app/automate/executor/actions/social/linkedin"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "LinkedIn Get Post"
	Description  = "Retrieve a LinkedIn post by its URN with engagement metrics"
	Website      = "https://www.flomation.co"
	Icon         = "linkedin"
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
	{Name: "text", Type: core.ConnectionTypeString, Label: "Post Text"},
	{Name: "author", Type: core.ConnectionTypeString, Label: "Author URN"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Created At"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "post_json", Type: core.ConnectionTypeString, Label: "Full Post JSON"},
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
	resp, err := linkedin.ExecuteAPI(token, "GET", linkedin.BaseURL+"/ugcPosts/"+encodedURN, nil)
	if err != nil {
		return linkedin.ErrorResult(fmt.Sprintf("failed to get post: %v", err)), nil
	}

	if err := linkedin.CheckResponse(resp); err != nil {
		return linkedin.ErrorResult(err.Error()), nil
	}

	var post map[string]interface{}
	if err := json.Unmarshal(resp.Body, &post); err != nil {
		return linkedin.ErrorResult(fmt.Sprintf("failed to parse post: %v", err)), nil
	}

	// Extract text from nested structure
	text := ""
	if sc, ok := post["specificContent"].(map[string]interface{}); ok {
		if share, ok := sc["com.linkedin.ugc.ShareContent"].(map[string]interface{}); ok {
			if commentary, ok := share["shareCommentary"].(map[string]interface{}); ok {
				if t, ok := commentary["text"].(string); ok {
					text = t
				}
			}
		}
	}

	author, _ := post["author"].(string)
	lifecycleState, _ := post["lifecycleState"].(string)

	createdAt := ""
	if created, ok := post["created"].(map[string]interface{}); ok {
		if t, ok := created["time"].(float64); ok {
			createdAt = fmt.Sprintf("%d", int64(t))
		}
	}

	postJSON, _ := json.Marshal(post)

	return linkedin.SuccessResult(
		fmt.Sprintf("Post by %s: %s", author, truncate(text, 80)),
		map[string]interface{}{
			"text":            text,
			"author":          author,
			"created_at":      createdAt,
			"lifecycle_state": lifecycleState,
			"post_json":       string(postJSON),
		},
	), nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
