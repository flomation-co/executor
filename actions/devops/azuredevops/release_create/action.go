package devops_azuredevops_release_create

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Create Release"
	Description  = "Create a classic release from a release pipeline. Note this is classic Release Management, which Microsoft has steered new work away from in favour of multi-stage YAML pipelines — if your project deploys from a pipeline, use Run Pipeline instead."
	Website      = "https://www.flomation.co"
	Icon         = "azure+plus"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "definition_id", Type: core.ConnectionTypeInteger, Label: "Release Pipeline", Placeholder: "the release definition ID", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "why this release is being created"},
	{Name: "artifacts", Type: core.ConnectionTypeObject, Label: "Artifacts", Placeholder: "[{\"alias\": \"_Build\", \"instanceReference\": {\"id\": \"1234\"}}] — blank uses each artifact's latest version"},
	{Name: "is_draft", Type: core.ConnectionTypeBoolean, Label: "Create as Draft", Placeholder: "create without starting any deployment"},
	{Name: "manual_environments", Type: core.ConnectionTypeString, Label: "Manual Stages", Placeholder: "comma-separated stage names to hold for a manual start, e.g. Production"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Release ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Release"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Release URL"},
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
	definitionID, err := azuredevops.RequiredInt("definition_id", "Release Pipeline", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{"definitionId": definitionID}
	azuredevops.SetIfPresent(body, inputs, "description", "description")
	azuredevops.SetBoolIfSet(body, inputs, "isDraft", "is_draft")
	if env := azuredevops.SplitCommaList(azuredevops.OptionalString("manual_environments", inputs)); len(env) > 0 {
		body["manualEnvironments"] = env
	}
	// Artifacts is an ARRAY, not an object, so it cannot go through
	// ObjectInput — a release pins each artifact to a specific build.
	artifacts, err := azuredevops.OptionalJSON("artifacts", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	if artifacts != nil {
		list, ok := artifacts.([]interface{})
		if !ok {
			return azuredevops.ErrorResult(`Artifacts must be a JSON array, e.g. [{"alias": "_Build", "instanceReference": {"id": "1234"}}]`), nil
		}
		body["artifacts"] = list
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodPost,
		Path:   azuredevops.ProjectPath(project) + "/_apis/release/releases",
		Body:   body,
		Host:   azuredevops.HostRelease,
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
	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Created release %s from pipeline %d", azuredevops.IDOf(obj), definitionID))
	out["url"], _ = obj["url"].(string)
	return out, nil
}
