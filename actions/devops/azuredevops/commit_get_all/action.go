package devops_azuredevops_commit_get_all

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: List Commits"
	Description  = "List commits in a repository, filterable by branch, author, date range and file path. Useful for building release notes or spotting what landed since a given date."
	Website      = "https://www.flomation.co"
	Icon         = "azure+code"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "repository", Type: core.ConnectionTypeString, Label: "Repository", Placeholder: "repository name or ID", Required: true},
	{Name: "branch", Type: core.ConnectionTypeString, Label: "Branch", Placeholder: "main — leave blank for the default branch"},
	{Name: "author", Type: core.ConnectionTypeString, Label: "Author", Placeholder: "the commit author's display name or email"},
	{Name: "from_date", Type: core.ConnectionTypeString, Label: "From Date", Placeholder: "only commits after this, e.g. 2026-07-01T00:00:00Z"},
	{Name: "to_date", Type: core.ConnectionTypeString, Label: "To Date", Placeholder: "only commits before this, e.g. 2026-07-17T00:00:00Z"},
	{Name: "item_path", Type: core.ConnectionTypeString, Label: "File Path", Placeholder: "only commits touching this path, e.g. /src/api"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "how many to return (default 50, max 1000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Commits"},
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
	// The branch goes in as a bare name under itemVersion.version — this is the
	// one Git endpoint that does NOT want a full refs/heads/ ref, so
	// FullRefName is deliberately not used here.
	if v := azuredevops.OptionalString("branch", inputs); v != "" {
		q.Set("searchCriteria.itemVersion.version", strings.TrimPrefix(v, "refs/heads/"))
	}
	if v := azuredevops.OptionalString("author", inputs); v != "" {
		q.Set("searchCriteria.author", v)
	}
	if v := azuredevops.OptionalString("from_date", inputs); v != "" {
		q.Set("searchCriteria.fromDate", v)
	}
	if v := azuredevops.OptionalString("to_date", inputs); v != "" {
		q.Set("searchCriteria.toDate", v)
	}
	if v := azuredevops.OptionalString("item_path", inputs); v != "" {
		q.Set("searchCriteria.itemPath", v)
	}
	limit, set := azuredevops.OptionalInt("limit", inputs)
	q.Set("searchCriteria.$top", strconv.Itoa(azuredevops.ClampLimit(limit, set)))

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   azuredevops.ProjectPath(project) + "/_apis/git/repositories" + azuredevops.ProjectPath(repo) + "/commits",
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
	return azuredevops.ListResult(items, fmt.Sprintf("Found %d commit(s) in %s", len(items), repo)), nil
}
