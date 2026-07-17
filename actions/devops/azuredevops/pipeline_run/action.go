package devops_azuredevops_pipeline_run

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Run Pipeline"
	Description  = "Queue a pipeline run — the headline action, and the reason to wire Azure DevOps into a flow at all. Returns immediately with the run in progress; pair it with Get Pipeline Run to wait for the outcome. Variables can be given as plain name/value pairs."
	Website      = "https://www.flomation.co"
	Icon         = "azure+play"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "pipeline_id", Type: core.ConnectionTypeInteger, Label: "Pipeline", Placeholder: "the pipeline ID (see List Pipelines)", Required: true},
	{Name: "branch", Type: core.ConnectionTypeString, Label: "Branch", Placeholder: "main — leave blank to use the pipeline's default branch"},
	{Name: "template_parameters", Type: core.ConnectionTypeObject, Label: "Template Parameters", Placeholder: "{\"environment\": \"staging\"} — the pipeline's declared parameters"},
	{Name: "variables", Type: core.ConnectionTypeObject, Label: "Variables", Placeholder: "{\"releaseTag\": \"v1.2.3\"} — only variables the pipeline marks settable at queue time"},
	{Name: "stages_to_skip", Type: core.ConnectionTypeString, Label: "Stages to Skip", Placeholder: "comma-separated stage names, e.g. Deploy,Notify"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Run ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Run"},
	{Name: "run_state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "run_url", Type: core.ConnectionTypeString, Label: "Run URL"},
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

	body := map[string]interface{}{}
	if branch := azuredevops.OptionalString("branch", inputs); branch != "" {
		// The pipelines API takes the branch as the self repository resource's
		// refName, and wants the FULL ref — "main" alone is a silent 400.
		body["resources"] = map[string]interface{}{
			"repositories": map[string]interface{}{
				"self": map[string]interface{}{"refName": azuredevops.FullRefName(branch)},
			},
		}
	}
	params, err := azuredevops.ObjectInput("template_parameters", "Template Parameters", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if params != nil {
		body["templateParameters"] = params
	}
	vars, err := azuredevops.ObjectInput("variables", "Variables", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if vars != nil {
		body["variables"] = azuredevops.NormalisePipelineVariables(vars)
	}
	if skip := azuredevops.SplitCommaList(azuredevops.OptionalString("stages_to_skip", inputs)); len(skip) > 0 {
		body["stagesToSkip"] = skip
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodPost,
		Path:   fmt.Sprintf("%s/_apis/pipelines/%d/runs", azuredevops.ProjectPath(project), pipelineID),
		Body:   body,
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
	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Queued run %s of pipeline %d", azuredevops.IDOf(obj), pipelineID))
	out["run_state"], _ = obj["state"].(string)
	if links, ok := obj["_links"].(map[string]interface{}); ok {
		if web, ok := links["web"].(map[string]interface{}); ok {
			out["run_url"], _ = web["href"].(string)
		}
	}
	if _, ok := out["run_url"]; !ok {
		out["run_url"] = ""
	}
	return out, nil
}
