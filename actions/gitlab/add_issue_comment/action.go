package gitlab_add_issue_comment

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Add Issue Comment"
	Description  = "Add a comment to a GitLab issue"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+comments"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "issue_iid", Type: core.ConnectionTypeString, Label: "Issue IID", Required: true},
	{Name: "body", Type: core.ConnectionTypeText, Label: "Comment Body", Placeholder: "Markdown comment text", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "comment_id", Type: core.ConnectionTypeString, Label: "Comment ID"},
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
	iid, err := gitlab.RequiredString("issue_iid", inputs)
	if err != nil {
		return nil, err
	}
	commentBody, err := gitlab.RequiredString("body", inputs)
	if err != nil {
		return nil, err
	}

	payload := map[string]interface{}{
		"body": commentBody,
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "POST", fmt.Sprintf("/issues/%s/notes", iid), payload)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to add comment: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var note struct {
		ID int `json:"id"`
	}
	if err := json.Unmarshal(resp.Body, &note); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Added comment %d to issue #%s", note.ID, iid),
		"comment_id":  fmt.Sprintf("%d", note.ID),
		"success":     true,
		"error":       "",
	}, nil
}
