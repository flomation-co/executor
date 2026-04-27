package gitlab_merge_merge_request

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Merge Merge Request"
	Description  = "Merge an approved merge request in a GitLab project"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "merge_request_iid", Type: core.ConnectionTypeString, Label: "Merge Request IID", Required: true},
	{Name: "squash", Type: core.ConnectionTypeBoolean, Label: "Squash Commits"},
	{Name: "should_remove_source_branch", Type: core.ConnectionTypeBoolean, Label: "Delete Source Branch"},
	{Name: "merge_when_pipeline_succeeds", Type: core.ConnectionTypeBoolean, Label: "Merge When Pipeline Succeeds"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
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

	body := map[string]interface{}{}
	if v := gitlab.OptionalBool("squash", inputs); v != nil {
		body["squash"] = *v
	}
	if v := gitlab.OptionalBool("should_remove_source_branch", inputs); v != nil {
		body["should_remove_source_branch"] = *v
	}
	if v := gitlab.OptionalBool("merge_when_pipeline_succeeds", inputs); v != nil {
		body["merge_when_pipeline_succeeds"] = *v
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "PUT", fmt.Sprintf("/merge_requests/%s/merge", iid), body)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to merge: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var mr struct {
		State  string `json:"state"`
		WebURL string `json:"web_url"`
		Title  string `json:"title"`
	}
	if err := json.Unmarshal(resp.Body, &mr); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Merged MR !%s: %s [%s] — %s", iid, mr.Title, mr.State, mr.WebURL),
		"state":       mr.State,
		"web_url":     mr.WebURL,
		"success":     true,
		"error":       "",
	}, nil
}
