package devops_azuredevops_build_cancel

import (
	"fmt"
	"net/http"
	"strconv"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Cancel Build"
	Description  = "Cancel an in-flight build. Cancellation is a request, not an instruction: the build moves to \"cancelling\" and finishes tearing down, so its final result arrives shortly afterwards. This is a Build API action because the Pipelines API has no cancel verb at all."
	Website      = "https://www.flomation.co"
	Icon         = "azure+circle-stop"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "build_id", Type: core.ConnectionTypeInteger, Label: "Build", Placeholder: "the build ID to cancel", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Build ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Build"},
	{Name: "build_status", Type: core.ConnectionTypeString, Label: "Status"},
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
		Method: http.MethodPatch,
		Path:   fmt.Sprintf("%s/_apis/build/builds/%d", azuredevops.ProjectPath(project), buildID),
		Body:   map[string]interface{}{"status": "cancelling"},
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
	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Requested cancellation of build %d", buildID))
	out["id"] = strconv.Itoa(buildID)
	out["build_status"] = status
	return out, nil
}
