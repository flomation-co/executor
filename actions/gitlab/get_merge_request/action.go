package gitlab_get_merge_request

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Get Merge Request"
	Description  = "Retrieve details of a GitLab merge request by IID"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+eye"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "merge_request_iid", Type: core.ConnectionTypeString, Label: "Merge Request IID", Placeholder: "The internal ID of the merge request", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "author", Type: core.ConnectionTypeString, Label: "Author"},
	{Name: "source_branch", Type: core.ConnectionTypeString, Label: "Source Branch"},
	{Name: "target_branch", Type: core.ConnectionTypeString, Label: "Target Branch"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "merge_status", Type: core.ConnectionTypeString, Label: "Merge Status"},
	{Name: "labels", Type: core.ConnectionTypeString, Label: "Labels (JSON)"},
	{Name: "data", Type: core.ConnectionTypeObject, Label: "Full Response (JSON)"},
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

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", fmt.Sprintf("/merge_requests/%s", iid), nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to get merge request: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var mr struct {
		Title        string   `json:"title"`
		Description  string   `json:"description"`
		State        string   `json:"state"`
		SourceBranch string   `json:"source_branch"`
		TargetBranch string   `json:"target_branch"`
		WebURL       string   `json:"web_url"`
		MergeStatus  string   `json:"merge_status"`
		Labels       []string `json:"labels"`
		Author       struct {
			Username string `json:"username"`
		} `json:"author"`
	}
	if err := json.Unmarshal(resp.Body, &mr); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	labelsJSON, _ := json.Marshal(mr.Labels)
	var fullData interface{}
	_ = json.Unmarshal(resp.Body, &fullData)

	summary := fmt.Sprintf("MR !%s: %s\nState: %s | Author: @%s\nBranch: %s → %s\nMerge Status: %s\nURL: %s\nLabels: %s\n",
		iid, mr.Title, mr.State, mr.Author.Username,
		mr.SourceBranch, mr.TargetBranch, mr.MergeStatus, mr.WebURL, string(labelsJSON))
	if mr.Description != "" {
		summary += fmt.Sprintf("\nDescription:\n%s\n", mr.Description)
	}

	return map[string]interface{}{
		"tool_result":   summary,
		"title":         mr.Title,
		"description":   mr.Description,
		"state":         mr.State,
		"author":        mr.Author.Username,
		"source_branch": mr.SourceBranch,
		"target_branch": mr.TargetBranch,
		"web_url":       mr.WebURL,
		"merge_status":  mr.MergeStatus,
		"labels":        string(labelsJSON),
		"data":          fullData,
		"success":       true,
		"error":         "",
	}, nil
}
