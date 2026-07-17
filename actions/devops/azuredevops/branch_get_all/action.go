package devops_azuredevops_branch_get_all

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
	Name         = "Azure DevOps: List Branches"
	Description  = "List a repository's branches, with each one's latest commit. Filter narrows by name prefix — \"release/\" returns only the release branches."
	Website      = "https://www.flomation.co"
	Icon         = "azure+code-branch"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID", Required: true},
	{Name: "repository", Type: core.ConnectionTypeString, Label: "Repository", Placeholder: "repository name or ID", Required: true},
	{Name: "filter", Type: core.ConnectionTypeString, Label: "Name Starts With", Placeholder: "release/ — leave blank for every branch"},
	{Name: "include_tags", Type: core.ConnectionTypeBoolean, Label: "Include Tags", Placeholder: "also return tags, not just branches"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "how many to return (default 50, max 1000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Branches"},
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
	// The refs endpoint filters by ref-name prefix with the "refs/" root already
	// implied, so branches are heads/ and tags are tags/ — the filter an
	// operator types is appended to that.
	prefix := "heads/"
	if azuredevops.OptionalBool("include_tags", inputs) {
		prefix = ""
	}
	q.Set("filter", prefix+strings.TrimPrefix(azuredevops.OptionalString("filter", inputs), "refs/"))
	limit, set := azuredevops.OptionalInt("limit", inputs)
	q.Set("$top", strconv.Itoa(azuredevops.ClampLimit(limit, set)))
	q.Set("latestStatusesOnly", "true")

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   azuredevops.ProjectPath(project) + "/_apis/git/repositories" + azuredevops.ProjectPath(repo) + "/refs",
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
	return azuredevops.ListResult(items, fmt.Sprintf("Found %d branch(es) in %s", len(items), repo)), nil
}
