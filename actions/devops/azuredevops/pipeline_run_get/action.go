package devops_azuredevops_pipeline_run_get

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Get Pipeline Run"
	Description  = "Get a pipeline run's state and result — the polling half of run, wait, report. State and Result are SEPARATE: a run is only finished when State is \"completed\", and until then Result reads \"unknown\", which does not mean it failed."
	Website      = "https://www.flomation.co"
	Icon         = "azure+eye"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "pipeline_id", Type: core.ConnectionTypeInteger, Label: "Pipeline", Placeholder: "the pipeline ID (see List Pipelines)", Required: true},
	{Name: "run_id", Type: core.ConnectionTypeInteger, Label: "Run", Placeholder: "the run ID returned by Run Pipeline", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Run ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Run"},
	{Name: "run_state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "run_result", Type: core.ConnectionTypeString, Label: "Result"},
	{Name: "finished", Type: core.ConnectionTypeBoolean, Label: "Finished"},
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
	runID, err := azuredevops.RequiredInt("run_id", "Run", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("%s/_apis/pipelines/%d/runs/%d", azuredevops.ProjectPath(project), pipelineID, runID),
	})
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if err := azuredevops.CheckResponse(resp); err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	obj, err := azuredevops.Decode(resp)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	state, _ := obj["state"].(string)
	result, _ := obj["result"].(string)
	// "finished" is lifted out so a flow can branch on one boolean instead of
	// re-deriving the state/result split every time. Checking Result alone —
	// the obvious thing to do — reports phantom failures on a running build.
	finished := state == "completed"
	summary := fmt.Sprintf("Run %d is %s", runID, state)
	if finished {
		summary = fmt.Sprintf("Run %d completed: %s", runID, result)
	}
	out := azuredevops.ResourceResult(obj, summary)
	out["run_state"] = state
	out["run_result"] = result
	out["finished"] = finished
	return out, nil
}
