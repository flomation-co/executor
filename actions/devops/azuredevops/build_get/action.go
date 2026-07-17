package devops_azuredevops_build_get

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Get Build"
	Description  = "Get one build by ID, including its status, result, requester and source branch."
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
	{Name: "build_id", Type: core.ConnectionTypeInteger, Label: "Build", Placeholder: "the build ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Build ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Build"},
	{Name: "build_status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "build_result", Type: core.ConnectionTypeString, Label: "Result"},
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
	buildID, err := azuredevops.RequiredInt("build_id", "Build", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("%s/_apis/build/builds/%d", azuredevops.ProjectPath(project), buildID),
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
	status, _ := obj["status"].(string)
	result, _ := obj["result"].(string)
	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Build %d is %s", buildID, status))
	out["build_status"] = status
	out["build_result"] = result
	return out, nil
}
