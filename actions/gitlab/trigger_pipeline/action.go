package gitlab_trigger_pipeline

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Trigger Pipeline"
	Description  = "Create and trigger a new pipeline for a branch or tag"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+play"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "ref", Type: core.ConnectionTypeString, Label: "Ref", Placeholder: "Branch or tag to run the pipeline on", Required: true},
	{Name: "variables", Type: core.ConnectionTypeKeyValueArray, Label: "Pipeline Variables"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pipeline_id", Type: core.ConnectionTypeString, Label: "Pipeline ID"},
	{Name: "web_url", Type: core.ConnectionTypeString, Label: "Web URL"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
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
	ref, err := gitlab.RequiredString("ref", inputs)
	if err != nil {
		return nil, err
	}

	body := map[string]interface{}{
		"ref": ref,
	}

	// Add pipeline variables if provided
	conn := core.FindConnection("variables", inputs)
	if conn != nil {
		pairs := conn.KeyValuePairs()
		if len(pairs) > 0 {
			vars := make([]map[string]string, 0, len(pairs))
			for _, p := range pairs {
				vars = append(vars, map[string]string{
					"key":           p.Key,
					"value":         p.Value,
					"variable_type": "env_var",
				})
			}
			body["variables"] = vars
		}
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "POST", "/pipeline", body)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to trigger pipeline: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var pipeline struct {
		ID     int    `json:"id"`
		WebURL string `json:"web_url"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(resp.Body, &pipeline); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Triggered pipeline %d on %s [%s] — %s", pipeline.ID, ref, pipeline.Status, pipeline.WebURL),
		"pipeline_id": fmt.Sprintf("%d", pipeline.ID),
		"web_url":     pipeline.WebURL,
		"status":      pipeline.Status,
		"success":     true,
		"error":       "",
	}, nil
}
