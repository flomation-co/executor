package linkedin_list_posts

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
	Name         = "LinkedIn List Posts"
	Description  = "List recent posts by the authenticated user or a specified author"
	Website      = "https://www.flomation.co"
	Icon         = "linkedin+list"
	Date         = "21/05/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "LinkedIn Access Token", Placeholder: "${credentials.linkedin_community}", Required: true},
	{Name: "author_urn", Type: core.ConnectionTypeString, Label: "Author URN", Placeholder: "urn:li:person:XXXXXXXX", Required: true},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count", Placeholder: "10"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
	{Name: "posts", Type: core.ConnectionTypeString, Label: "Posts (JSON array)"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total Posts"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := linkedin.GetAccessToken(inputs)
	if err != nil {
		return linkedin.ErrorResult(err.Error()), nil
	}

	authorURN := linkedin.OptionalString("author_urn", inputs)
	if authorURN == "" {
		return linkedin.ErrorResult("author_urn is required"), nil
	}

	count := "10"
	if c := linkedin.OptionalString("count", inputs); c != "" {
		count = c
	}

	params := url.Values{
		"q":      {"author"},
		"author": {authorURN},
		"count":  {count},
	}

	apiURL := linkedin.RestBaseURL + "/posts?" + params.Encode()
	resp, err := linkedin.ExecuteVersionedAPI(token, "GET", apiURL, nil)
	if err != nil {
		return linkedin.ErrorResult(fmt.Sprintf("failed to list posts: %v", err)), nil
	}

	if err := linkedin.CheckResponse(resp); err != nil {
		return linkedin.ErrorResult(err.Error()), nil
	}

	var result struct {
		Elements []map[string]interface{} `json:"elements"`
		Paging   struct {
			Total int64 `json:"total"`
			Count int64 `json:"count"`
		} `json:"paging"`
	}
	if err := json.Unmarshal(resp.Body, &result); err != nil {
		return linkedin.ErrorResult(fmt.Sprintf("failed to parse posts: %v", err)), nil
	}

	postsJSON, _ := json.Marshal(result.Elements)

	return linkedin.SuccessResult(
		fmt.Sprintf("Found %d posts", len(result.Elements)),
		map[string]interface{}{
			"posts": string(postsJSON),
			"total": int64(len(result.Elements)),
		},
	), nil
}
