package linkedin_get_analytics

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
	Name         = "LinkedIn Post Analytics"
	Description  = "Get engagement metrics (likes, comments, shares) for a LinkedIn post"
	Website      = "https://www.flomation.co"
	Icon         = "linkedin+pie-chart"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "LinkedIn Access Token", Placeholder: "${credentials.linkedin_community}", Required: true},
	{Name: "post_urn", Type: core.ConnectionTypeString, Label: "Post URN", Placeholder: "urn:li:share:...", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
	{Name: "likes", Type: core.ConnectionTypeInteger, Label: "Likes"},
	{Name: "comments", Type: core.ConnectionTypeInteger, Label: "Comments"},
	{Name: "shares", Type: core.ConnectionTypeInteger, Label: "Shares"},
	{Name: "analytics_json", Type: core.ConnectionTypeString, Label: "Full Analytics JSON"},
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

	encodedURN := url.QueryEscape(postURN)
	apiURL := fmt.Sprintf("%s/socialMetadata/%s", linkedin.RestBaseURL, encodedURN)

	resp, err := linkedin.ExecuteVersionedAPI(token, "GET", apiURL, nil)
	if err != nil {
		return linkedin.ErrorResult(fmt.Sprintf("failed to get analytics: %v", err)), nil
	}

	if err := linkedin.CheckResponse(resp); err != nil {
		return linkedin.ErrorResult(err.Error()), nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal(resp.Body, &metadata); err != nil {
		return linkedin.ErrorResult(fmt.Sprintf("failed to parse analytics: %v", err)), nil
	}

	likes := extractCount(metadata, "likesSummary", "totalLikes")
	comments := extractCount(metadata, "commentsSummary", "totalFirstLevelComments")
	shares := extractCount(metadata, "sharesSummary", "totalShares")

	analyticsJSON, _ := json.Marshal(metadata)

	return linkedin.SuccessResult(
		fmt.Sprintf("Likes: %d, Comments: %d, Shares: %d", likes, comments, shares),
		map[string]interface{}{
			"likes":          likes,
			"comments":       comments,
			"shares":         shares,
			"analytics_json": string(analyticsJSON),
		},
	), nil
}

func extractCount(data map[string]interface{}, summaryKey, countKey string) int64 {
	if summary, ok := data[summaryKey].(map[string]interface{}); ok {
		if count, ok := summary[countKey].(float64); ok {
			return int64(count)
		}
	}
	return 0
}
