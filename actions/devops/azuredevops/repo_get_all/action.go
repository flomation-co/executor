package devops_azuredevops_repo_get_all

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
	Name         = "Azure DevOps: List Repositories"
	Description  = "List the Git repositories in a project, with each one's ID, default branch and clone URLs."
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
	{Name: "include_hidden", Type: core.ConnectionTypeBoolean, Label: "Include Hidden", Placeholder: "also return repositories hidden from the project's repo list"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "how many to return (default 50, max 1000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Repositories"},
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

	q := url.Values{}
	if azuredevops.OptionalBool("include_hidden", inputs) {
		q.Set("includeHidden", "true")
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   azuredevops.ProjectPath(project) + "/_apis/git/repositories",
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

	// The repositories endpoint returns the project's whole set and takes no
	// paging parameters, so Limit trims on our side rather than pretending.
	limit, set := azuredevops.OptionalInt("limit", inputs)
	if n := azuredevops.ClampLimit(limit, set); len(items) > n {
		items = items[:n]
	}
	return azuredevops.ListResult(items, fmt.Sprintf("Found %d repositor(y/ies) in %s", len(items), project)), nil
}
