package linkedin_share_post

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	linkedin "flomation.app/automate/executor/actions/social/linkedin"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "LinkedIn Share Post"
	Description  = "Publish a text or link post to LinkedIn as the authenticated user"
	Website      = "https://www.flomation.co"
	Icon         = "linkedin"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "LinkedIn Access Token", Placeholder: "${credentials.linkedin}", Required: true},
	{Name: "author_urn", Type: core.ConnectionTypeString, Label: "Author URN", Placeholder: "urn:li:person:XXXXXXXX", Required: true},
	{Name: "text", Type: core.ConnectionTypeText, Label: "Post Text", Required: true},
	{Name: "link_url", Type: core.ConnectionTypeString, Label: "Link URL", Placeholder: "https://example.com (optional)"},
	{Name: "link_title", Type: core.ConnectionTypeString, Label: "Link Title"},
	{Name: "link_description", Type: core.ConnectionTypeString, Label: "Link Description"},
	{Name: "visibility", Type: core.ConnectionTypeString, Label: "Visibility", Placeholder: "PUBLIC",
		Options: []core.ConnectionOption{
			{Name: "Public", Value: "PUBLIC"},
			{Name: "Connections Only", Value: "CONNECTIONS"},
		},
	},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
	{Name: "post_urn", Type: core.ConnectionTypeString, Label: "Post URN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := linkedin.GetAccessToken(inputs)
	if err != nil {
		return linkedin.ErrorResult(err.Error()), nil
	}

	authorURN := linkedin.OptionalString("author_urn", inputs)
	if authorURN == "" {
		return linkedin.ErrorResult("author_urn is required — use LinkedIn Get Profile to obtain it"), nil
	}

	text := linkedin.OptionalString("text", inputs)
	if text == "" {
		return linkedin.ErrorResult("text is required"), nil
	}

	visibility := linkedin.OptionalString("visibility", inputs)
	if visibility == "" {
		visibility = "PUBLIC"
	}

	// Build the UGC post payload
	post := map[string]interface{}{
		"author":         authorURN,
		"lifecycleState": "PUBLISHED",
		"visibility": map[string]interface{}{
			"com.linkedin.ugc.MemberNetworkVisibility": visibility,
		},
		"specificContent": map[string]interface{}{
			"com.linkedin.ugc.ShareContent": buildShareContent(text, inputs),
		},
	}

	resp, err := linkedin.ExecuteAPI(token, "POST", linkedin.BaseURL+"/ugcPosts", post)
	if err != nil {
		return linkedin.ErrorResult(fmt.Sprintf("failed to share post: %v", err)), nil
	}

	if err := linkedin.CheckResponse(resp); err != nil {
		return linkedin.ErrorResult(err.Error()), nil
	}

	// Extract post URN from response
	var result struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(resp.Body, &result)

	// Also check X-RestLi-Id header
	if result.ID == "" {
		result.ID = resp.Headers.Get("X-RestLi-Id")
	}

	return linkedin.SuccessResult(
		fmt.Sprintf("Post shared successfully: %s", result.ID),
		map[string]interface{}{
			"post_urn": result.ID,
		},
	), nil
}

func buildShareContent(text string, inputs []*core.Connection) map[string]interface{} {
	content := map[string]interface{}{
		"shareCommentary": map[string]interface{}{
			"text": text,
		},
		"shareMediaCategory": "NONE",
	}

	linkURL := linkedin.OptionalString("link_url", inputs)
	if linkURL != "" {
		media := map[string]interface{}{
			"status":      "READY",
			"originalUrl": linkURL,
		}
		title := linkedin.OptionalString("link_title", inputs)
		if title != "" {
			media["title"] = map[string]interface{}{"text": title}
		}
		desc := linkedin.OptionalString("link_description", inputs)
		if desc != "" {
			media["description"] = map[string]interface{}{"text": desc}
		}
		content["shareMediaCategory"] = "ARTICLE"
		content["media"] = []interface{}{media}
	}

	return content
}
