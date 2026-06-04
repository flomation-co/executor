package gitlab_get_job_log

import (
	"fmt"

	core "flomation.app/automate/executor"
	gitlab "flomation.app/automate/executor/actions/gitlab"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "GitLab Get Job Log"
	Description  = "Retrieve the log/trace output of a GitLab CI/CD job"
	Website      = "https://www.flomation.co"
	Icon         = "gitlab+file-lines"
	Date         = "27/04/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeString, Label: "GitLab Access Token", Placeholder: "glpat-...", Required: true},
	{Name: "base_url", Type: core.ConnectionTypeString, Label: "GitLab Base URL", Placeholder: "https://gitlab.com"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project ID", Placeholder: "Numeric ID or namespace/project", Required: true},
	{Name: "job_id", Type: core.ConnectionTypeString, Label: "Job ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "log", Type: core.ConnectionTypeString, Label: "Full Log Output"},
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
	jobID, err := gitlab.RequiredString("job_id", inputs)
	if err != nil {
		return nil, err
	}

	// GitLab returns job trace as plain text, not JSON
	resp, err := gitlab.ProjectAPI(token, baseURL, projectID, "GET", fmt.Sprintf("/jobs/%s/trace", jobID), nil)
	if err != nil {
		return gitlab.ErrorResult(fmt.Sprintf("Failed to get job log: %s", err)), nil
	}
	if err := gitlab.CheckResponse(resp); err != nil {
		return gitlab.ErrorResult(err.Error()), nil
	}

	logText := string(resp.Body)

	// For tool_result, include the full log — the AI needs to read it
	// to diagnose failures. Truncate only if extremely large.
	summary := logText
	if len(summary) > 16000 {
		// Keep the last 16000 chars — the end of a build log is usually
		// where the error is.
		summary = "... (truncated, showing last 16000 chars) ...\n" + summary[len(summary)-16000:]
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Job %s log (%d bytes):\n\n%s", jobID, len(logText), summary),
		"log":         logText,
		"success":     true,
		"error":       "",
	}, nil
}
