package devops_azuredevops_release_get_all

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
	Name         = "Azure DevOps: List Releases"
	Description  = "List classic releases in a project. Note this is classic Release Management, which Microsoft has steered new work away from in favour of multi-stage YAML pipelines — if your project deploys from a pipeline, use List Pipeline Runs instead."
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
	{Name: "definition_id", Type: core.ConnectionTypeInteger, Label: "Release Pipeline", Placeholder: "a release definition ID — blank for every release in the project"},
	{Name: "status_filter", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "blank for any status", Options: []core.ConnectionOption{{Name: "Active", Value: "active"}, {Name: "Draft", Value: "draft"}, {Name: "Abandoned", Value: "abandoned"}}},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "how many to return (default 50, max 1000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Releases"},
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
	if id, set := azuredevops.OptionalInt("definition_id", inputs); set {
		q.Set("definitionId", strconv.Itoa(id))
	}
	if v := azuredevops.OptionalString("status_filter", inputs); v != "" {
		q.Set("statusFilter", v)
	}
	limit, set := azuredevops.OptionalInt("limit", inputs)
	q.Set("$top", strconv.Itoa(azuredevops.ClampLimit(limit, set)))

	// HostRelease is not decoration: classic releases are served from
	// vsrm.dev.azure.com, and this exact call against dev.azure.com 404s.
	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   azuredevops.ProjectPath(project) + "/_apis/release/releases",
		Query:  q,
		Host:   azuredevops.HostRelease,
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
	return azuredevops.ListResult(items, fmt.Sprintf("Found %d release(s) in %s", len(items), project)), nil
}
