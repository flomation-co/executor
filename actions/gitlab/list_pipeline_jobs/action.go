package gitlab_list_pipeline_jobs

import (
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab List Pipeline Jobs"
	Description  = "List jobs within a GitLab pipeline"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "pipeline_id", Type: core.ConnectionTypeString, Label: "Pipeline ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "jobs", Type: core.ConnectionTypeObject, Label: "Jobs (JSON)"},
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
	pipelineID, err := gitlab.RequiredString("pipeline_id", inputs)
	if err != nil {
		return nil, err
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", fmt.Sprintf("/pipelines/%s/jobs", pipelineID), nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to list jobs: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var jobs []interface{}
	if err := json.Unmarshal(resp.Body, &jobs); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	summary := fmt.Sprintf("Pipeline %s has %d job(s):\n", pipelineID, len(jobs))
	for _, j := range jobs {
		if jm, ok := j.(map[string]interface{}); ok {
			summary += fmt.Sprintf("- %v: %v [%v] stage=%v\n", jm["id"], jm["name"], jm["status"], jm["stage"])
		}
	}

	return map[string]interface{}{
		"tool_result": summary,
		"jobs":        jobs,
		"count":       int64(len(jobs)),
		"success":     true,
		"error":       "",
	}, nil
}
