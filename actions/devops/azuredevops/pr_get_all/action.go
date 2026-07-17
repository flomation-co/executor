package devops_azuredevops_pr_get_all

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: List Pull Requests"
	Description  = "List pull requests in a repository, filterable by status, creator, reviewer and target branch. \"PRs awaiting review\" is the classic use — set Status to Active and give a Reviewer ID."
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
	{Name: "repository", Type: core.ConnectionTypeString, Label: "Repository", Placeholder: "repository name or ID", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "default active", Options: []core.ConnectionOption{{Name: "Active", Value: "active"}, {Name: "Completed", Value: "completed"}, {Name: "Abandoned", Value: "abandoned"}, {Name: "All", Value: "all"}}},
	{Name: "creator_id", Type: core.ConnectionTypeString, Label: "Creator ID", Placeholder: "identity ID (GUID) of the PR author"},
	{Name: "reviewer_id", Type: core.ConnectionTypeString, Label: "Reviewer ID", Placeholder: "identity ID (GUID) of a reviewer"},
	{Name: "target_branch", Type: core.ConnectionTypeString, Label: "Target Branch", Placeholder: "main — only PRs merging into this branch"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "how many to return (default 50, max 1000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Pull Requests"},
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
	repo, err := azuredevops.RequiredString("repository", "Repository", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	status := azuredevops.OptionalString("status", inputs)
	if status == "" {
		status = "active"
	}
	q.Set("searchCriteria.status", status)
	if v := azuredevops.OptionalString("creator_id", inputs); v != "" {
		q.Set("searchCriteria.creatorId", v)
	}
	if v := azuredevops.OptionalString("reviewer_id", inputs); v != "" {
		q.Set("searchCriteria.reviewerId", v)
	}
	if v := azuredevops.OptionalString("target_branch", inputs); v != "" {
		q.Set("searchCriteria.targetRefName", azuredevops.FullRefName(v))
	}
	// Pull requests page with $top/$skip, not a continuation token, so this is
	// a single bounded page rather than a Return All walk.
	limit, set := azuredevops.OptionalInt("limit", inputs)
	q.Set("$top", strconv.Itoa(azuredevops.ClampLimit(limit, set)))

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   azuredevops.ProjectPath(project) + "/_apis/git/repositories" + azuredevops.ProjectPath(repo) + "/pullrequests",
		Query:  q,
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
	return azuredevops.ListResult(items, fmt.Sprintf("Found %d %s pull request(s)", len(items), status)), nil
}
