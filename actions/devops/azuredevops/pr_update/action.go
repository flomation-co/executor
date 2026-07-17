package devops_azuredevops_pr_update

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
	Name         = "Azure DevOps: Update Pull Request"
	Description  = "Update a pull request's title, description or target branch — and this is also how you abandon one (set Status to Abandoned) or bring it back (Active). Publishing a draft is Draft = off. To merge, use Complete Pull Request."
	Website      = "https://www.flomation.co"
	Icon         = "azure+pen"
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
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "leave blank to keep the current title"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "leave blank to keep the current description"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "leave blank to keep the current status", Options: []core.ConnectionOption{{Name: "Active (reactivate)", Value: "active"}, {Name: "Abandoned", Value: "abandoned"}}},
	{Name: "target_branch", Type: core.ConnectionTypeString, Label: "Target Branch", Placeholder: "retarget the PR at another branch, e.g. release/2.0"},
	{Name: "is_draft", Type: core.ConnectionTypeBoolean, Label: "Draft", Placeholder: "turn off to publish a draft PR and start the review"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Pull Request ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Pull Request"},
	{Name: "pr_status", Type: core.ConnectionTypeString, Label: "Status"},
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

	body := map[string]interface{}{}
	azuredevops.SetIfPresent(body, inputs, "title", "title")
	azuredevops.SetIfPresent(body, inputs, "description", "description")
	azuredevops.SetIfPresent(body, inputs, "status", "status")
	azuredevops.SetBoolIfSet(body, inputs, "isDraft", "is_draft")
	if v := azuredevops.OptionalString("target_branch", inputs); v != "" {
		body["targetRefName"] = azuredevops.FullRefName(v)
	}
	if len(body) == 0 {
		return azuredevops.ErrorResult("nothing to update — set at least one of Title, Description, Status, Target Branch or Draft"), nil
	}
	// Completing a PR needs lastMergeSourceCommit echoed back, which this action
	// deliberately does not do — Complete Pull Request exists for that and does
	// the two-call dance properly. Allowing "completed" here would fail with a
	// confusing merge error.
	if status, _ := body["status"].(string); status == "completed" {
		return azuredevops.ErrorResult("use Complete Pull Request to merge — it supplies the merge options and the concurrency check this action cannot"), nil
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodPatch,
		Path:   fmt.Sprintf("%s/_apis/git/repositories%s/pullrequests/%d", azuredevops.ProjectPath(project), azuredevops.ProjectPath(repo), prID),
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
	status, _ := obj["status"].(string)
	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Updated pull request %d (%s)", prID, status))
	out["id"] = strconv.Itoa(prID)
	out["pr_status"] = status
	return out, nil
}
