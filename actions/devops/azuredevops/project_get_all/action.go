package devops_azuredevops_project_get_all

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: List Projects"
	Description  = "List the projects in an Azure DevOps organisation. Every other action needs a project, so this is the natural first step — and the connection test: it validates the Organisation URL and the Personal Access Token without touching anything else."
	Website      = "https://www.flomation.co"
	Icon         = "azure+list"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "state_filter", Type: core.ConnectionTypeString, Label: "State", Placeholder: "which projects to include — default well-formed (the normal, usable ones)", Options: []core.ConnectionOption{{Name: "Well Formed", Value: "wellFormed"}, {Name: "Create Pending", Value: "createPending"}, {Name: "Deleting", Value: "deleting"}, {Name: "All", Value: "all"}}},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "follow continuation tokens until every page is fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "how many to return (default 50, max 1000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Projects"},
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
	q := url.Values{}
	if state := azuredevops.OptionalString("state_filter", inputs); state != "" {
		q.Set("stateFilter", state)
	}
	returnAll := azuredevops.ApplyPaging(q, inputs)

	items, capped, err := azuredevops.ListAll(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   "/_apis/projects",
		Query:  q,
	}, returnAll)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	return azuredevops.ListResult(items, azuredevops.ListSummary("project", len(items), returnAll, capped)), nil
}
