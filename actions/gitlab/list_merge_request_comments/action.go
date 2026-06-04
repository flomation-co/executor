package gitlab_list_merge_request_comments

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab List MR Comments"
	Description  = "List notes and comments on a GitLab merge request"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+comments"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "merge_request_iid", Type: core.ConnectionTypeString, Label: "Merge Request IID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "comments", Type: core.ConnectionTypeObject, Label: "Comments (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := gitlab.GetAccessToken(inputs)
	if err != nil {
		return nil, err
	}
	baseURL := gitlab.GetBaseURL(inputs)
	projectID, err := gitlab.GetProjectID(inputs)
	if err != nil {
		return nil, err
	}
	iid, err := gitlab.RequiredString("merge_request_iid", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", fmt.Sprintf("/merge_requests/%s/notes", iid), nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to list comments: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var notes []interface{}
	if err := json.Unmarshal(resp.Body, &notes); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	summary := fmt.Sprintf("MR !%s has %d comment(s):\n", iid, len(notes))
	for _, n := range notes {
		if nm, ok := n.(map[string]interface{}); ok {
			author := ""
			if a, ok := nm["author"].(map[string]interface{}); ok {
				author, _ = a["username"].(string)
			}
			body, _ := nm["body"].(string)
			if len(body) > 100 {
				body = body[:100] + "..."
			}
			summary += fmt.Sprintf("- @%v: %v\n", author, body)
		}
	}

	return map[string]interface{}{
		"tool_result": summary,
		"comments":    notes,
		"count":       int64(len(notes)),
		"success":     true,
		"error":       "",
	}, nil
}
