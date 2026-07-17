package devops_azuredevops_pipeline_artifact_get

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Get Pipeline Artifact"
	Description  = "Get a named artifact from a pipeline run, with a time-limited download URL. Artifacts are how a pipeline hands its output to the rest of a flow. The URL is returned rather than the bytes — build artifacts are routinely gigabytes."
	Website      = "https://www.flomation.co"
	Icon         = "azure+box-archive"
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
	{Name: "artifact_name", Type: core.ConnectionTypeString, Label: "Artifact Name", Placeholder: "the artifact's publish name, e.g. drop", Required: true},
	{Name: "signed_content", Type: core.ConnectionTypeBoolean, Label: "Include Download URL", Placeholder: "return a time-limited anonymous download URL (on by default)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Artifact Name"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Artifact"},
	{Name: "download_url", Type: core.ConnectionTypeString, Label: "Download URL"},
	{Name: "expires_at", Type: core.ConnectionTypeString, Label: "URL Expires At"},
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
	name, err := azuredevops.RequiredString("artifact_name", "Artifact Name", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	q.Set("artifactName", name)
	if azuredevops.BoolOrDefault("signed_content", inputs, true) {
		q.Set("$expand", "signedContent")
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("%s/_apis/pipelines/%d/runs/%d/artifacts", azuredevops.ProjectPath(project), pipelineID, runID),
		Query:  q,
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

	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Fetched artifact %q from run %d", name, runID))
	// The artifact has no id field; its name is its identity.
	if n, ok := obj["name"].(string); ok && n != "" {
		out["id"] = n
	} else {
		out["id"] = name
	}
	out["download_url"] = ""
	out["expires_at"] = ""
	if signed, ok := obj["signedContent"].(map[string]interface{}); ok {
		out["download_url"], _ = signed["url"].(string)
		out["expires_at"], _ = signed["signatureExpires"].(string)
	}
	return out, nil
}
