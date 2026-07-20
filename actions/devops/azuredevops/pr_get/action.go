package devops_azuredevops_pr_get

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
	Name         = "Azure DevOps: Get Pull Request"
	Description  = "Get one pull request, including its merge status, reviewers and the last source commit. Merge Status is what tells you whether it can actually be completed."
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
	{Name: "repository", Type: core.ConnectionTypeString, Label: "Repository", Placeholder: "repository name or ID", Required: true},
	{Name: "pull_request_id", Type: core.ConnectionTypeInteger, Label: "Pull Request", Placeholder: "the PR ID, e.g. 128", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Pull Request ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Pull Request"},
	{Name: "pr_status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "merge_status", Type: core.ConnectionTypeString, Label: "Merge Status"},
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
	repo, err := azuredevops.RequiredString("repository", "Repository", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	prID, err := azuredevops.RequiredInt("pull_request_id", "Pull Request", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   fmt.Sprintf("%s/_apis/git/repositories%s/pullrequests/%d", azuredevops.ProjectPath(project), azuredevops.ProjectPath(repo), prID),
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
	mergeStatus, _ := obj["mergeStatus"].(string)
	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Fetched pull request %d (%s)", prID, status))
	out["id"] = strconv.Itoa(prID)
	out["pr_status"] = status
	out["merge_status"] = mergeStatus
	return out, nil
}
