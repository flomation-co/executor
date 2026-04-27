package gitlab_list_pipelines

import (
	"encoding/json"
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab List Pipelines"
	Description  = "List pipelines for a GitLab project with optional filters"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab"
	Date         = "26/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "ref", Type: core.ConnectionTypeString, Label: "Ref", Placeholder: "Branch or tag name"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{
		{Name: "All", Value: ""},
		{Name: "Running", Value: "running"},
		{Name: "Pending", Value: "pending"},
		{Name: "Success", Value: "success"},
		{Name: "Failed", Value: "failed"},
		{Name: "Cancelled", Value: "canceled"},
	}},
	{Name: "per_page", Type: core.ConnectionTypeString, Label: "Per Page", Placeholder: "20 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "pipelines", Type: core.ConnectionTypeObject, Label: "Pipelines (JSON)"},
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

	params := url.Values{}
	if v := gitlab.OptionalString("ref", inputs); v != "" {
		params.Set("ref", v)
	}
	if v := gitlab.OptionalString("status", inputs); v != "" {
		params.Set("status", v)
	}
	if v := gitlab.OptionalString("per_page", inputs); v != "" {
		params.Set("per_page", v)
	}

	path := "/pipelines"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", path, nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to list pipelines: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	var pipelines []interface{}
	if err := json.Unmarshal(resp.Body, &pipelines); err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to parse response: %s", err)), nil
	}

	summary := fmt.Sprintf("Found %d pipeline(s):\n", len(pipelines))
	for _, p := range pipelines {
		if pm, ok := p.(map[string]interface{}); ok {
			summary += fmt.Sprintf("- #%v: %v on %v — %v\n", pm["id"], pm["status"], pm["ref"], pm["web_url"])
		}
	}

	return map[string]interface{}{
		"tool_result": summary,
		"pipelines":   pipelines,
		"count":       int64(len(pipelines)),
		"success":     true,
		"error":       "",
	}, nil
}
