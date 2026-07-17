package devops_azuredevops_pr_create

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
	Name         = "Azure DevOps: Create Pull Request"
	Description  = "Open a pull request. Branch names can be given plainly (\"main\") — the full ref Azure DevOps insists on is filled in for you. Reviewers are given as identity IDs; Work Item IDs link the PR to the work it delivers."
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
	{Name: "repository", Type: core.ConnectionTypeString, Label: "Repository", Placeholder: "repository name or ID", Required: true},
	{Name: "source_branch", Type: core.ConnectionTypeString, Label: "Source Branch", Placeholder: "the branch with your changes, e.g. feature/checkout-fix", Required: true},
	{Name: "target_branch", Type: core.ConnectionTypeString, Label: "Target Branch", Placeholder: "the branch to merge into, e.g. main", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "a short summary of the change", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "what changed and why"},
	{Name: "reviewers", Type: core.ConnectionTypeString, Label: "Reviewers", Placeholder: "comma-separated identity IDs (GUIDs) — not email addresses"},
	{Name: "work_item_ids", Type: core.ConnectionTypeString, Label: "Link Work Items", Placeholder: "comma-separated work item IDs to link, e.g. 42,43"},
	{Name: "is_draft", Type: core.ConnectionTypeBoolean, Label: "Draft", Placeholder: "open as a draft — no reviewers are notified and policies do not run"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Pull Request ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Pull Request"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Pull Request URL"},
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
	source, err := azuredevops.RequiredString("source_branch", "Source Branch", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	target, err := azuredevops.RequiredString("target_branch", "Target Branch", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	title, err := azuredevops.RequiredString("title", "Title", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	// FullRefName is why "main" works here: the Git APIs take refs/heads/main
	// and answer a bare branch name with a 400 that does not explain itself.
	body := map[string]interface{}{
		"sourceRefName": azuredevops.FullRefName(source),
		"targetRefName": azuredevops.FullRefName(target),
		"title":         title,
	}
	azuredevops.SetIfPresent(body, inputs, "description", "description")
	azuredevops.SetBoolIfSet(body, inputs, "isDraft", "is_draft")

	if ids := azuredevops.SplitCommaList(azuredevops.OptionalString("reviewers", inputs)); len(ids) > 0 {
		reviewers := make([]interface{}, 0, len(ids))
		for _, id := range ids {
			reviewers = append(reviewers, map[string]interface{}{"id": id})
		}
		body["reviewers"] = reviewers
	}
	if raw := azuredevops.OptionalString("work_item_ids", inputs); raw != "" {
		ids, err := azuredevops.ParseIDList(raw, "Link Work Items")
		if err != nil {
			return azuredevops.ErrorResult(err.Error()), nil
		}
		refs := make([]interface{}, 0, len(ids))
		for _, id := range ids {
			refs = append(refs, map[string]interface{}{"id": strconv.Itoa(id)})
		}
		body["workItemRefs"] = refs
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodPost,
		Path:   azuredevops.ProjectPath(project) + "/_apis/git/repositories" + azuredevops.ProjectPath(repo) + "/pullrequests",
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

	out := azuredevops.ResourceResult(obj, "")
	// A PR's identity is pullRequestId; the generic "id" field is absent, so
	// ResourceResult's lift finds nothing to use.
	out["id"] = azuredevops.IDOf(map[string]interface{}{"id": obj["pullRequestId"]})
	out["tool_result"] = fmt.Sprintf("Opened pull request %s: %s", out["id"], title)
	out["url"], _ = obj["url"].(string)
	return out, nil
}
