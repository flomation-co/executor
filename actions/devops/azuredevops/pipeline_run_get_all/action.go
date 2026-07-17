package devops_azuredevops_pipeline_run_get_all

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: List Pipeline Runs"
	Description  = "List the recent runs of one pipeline, newest first — for \"did today's build pass?\" style flows. The service returns its own recent window and takes no paging parameters, so Limit trims the list on our side."
	Website      = "https://www.flomation.co"
	Icon         = "azure+list"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "pipeline_id", Type: core.ConnectionTypeInteger, Label: "Pipeline", Placeholder: "the pipeline ID (see List Pipelines)", Required: true},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "how many to return (default 50, max 1000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Runs"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := azuredevops.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	project, err := azuredevops.RequiredString("project", "Project", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	pipelineID, err := azuredevops.RequiredInt("pipeline_id", "Pipeline", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("%s/_apis/pipelines/%d/runs", azuredevops.ProjectPath(project), pipelineID),
	})
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if err := azuredevops.CheckResponse(resp); err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	items, err := azuredevops.DecodeList(resp)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	// The runs endpoint honours neither $top nor a continuation token — it
	// simply returns its recent window. Trimming here keeps Limit meaning what
	// it says on every other list action instead of quietly doing nothing.
	limit, set := azuredevops.OptionalInt("limit", inputs)
	if n := azuredevops.ClampLimit(limit, set); len(items) > n {
		items = items[:n]
	}
	return azuredevops.ListResult(items, fmt.Sprintf("Found %d run(s) of pipeline %d", len(items), pipelineID)), nil
}
