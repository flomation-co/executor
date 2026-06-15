package gitlab_get_pipeline

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Get Pipeline"
	Description  = "Retrieve details of a specific GitLab pipeline"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+eye"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "pipeline_id", Type: core.ConnectionTypeString, Label: "Pipeline ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "ref", Type: core.ConnectionTypeString, Label: "Ref"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "created_at", Type: core.ConnectionTypeString, Label: "Created At"},
	{Name: "duration", Type: core.ConnectionTypeInteger, Label: "Duration (seconds)"},
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
	pipelineID, err := gitlab.RequiredString("pipeline_id", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", fmt.Sprintf("/pipelines/%s", pipelineID), nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to get pipeline: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var pipeline struct {
		Status    string  `json:"status"`
		Ref       string  `json:"ref"`
		WebURL    string  `json:"web_url"`
		CreatedAt string  `json:"created_at"`
		Duration  float64 `json:"duration"`
	}
	if err := json.Unmarshal(resp.Body, &pipeline); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	var fullData interface{}
	_ = json.Unmarshal(resp.Body, &fullData)

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Pipeline %s:\nStatus: %s\nRef: %s\nCreated: %s\nDuration: %.0fs\nURL: %s", pipelineID, pipeline.Status, pipeline.Ref, pipeline.CreatedAt, pipeline.Duration, pipeline.WebURL),
		"status":      pipeline.Status,
		"ref":         pipeline.Ref,
		"web_url":     pipeline.WebURL,
		"created_at":  pipeline.CreatedAt,
		"duration":    int64(pipeline.Duration),
		"data":        fullData,
		"success":     true,
		"error":       "",
	}, nil
}
