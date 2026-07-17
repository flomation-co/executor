package devops_azuredevops_build_get_all

import (
	"net/http"
	"net/url"
	"strings"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: List Builds"
	Description  = "List builds across a project, filterable by definition, status, result and branch. This is the classic Build API rather than the Pipelines API — deliberately: the Pipelines API can only list runs of ONE pipeline, so a cross-pipeline view is only available here."
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
	{Name: "definition_ids", Type: core.ConnectionTypeString, Label: "Pipeline IDs", Placeholder: "comma-separated pipeline/definition IDs, e.g. 12,14 — blank for all"},
	{Name: "status_filter", Type: core.ConnectionTypeString, Label: "Status", Placeholder: "blank for any status", Options: []core.ConnectionOption{{Name: "In Progress", Value: "inProgress"}, {Name: "Completed", Value: "completed"}, {Name: "Cancelling", Value: "cancelling"}, {Name: "Postponed", Value: "postponed"}, {Name: "Not Started", Value: "notStarted"}}},
	{Name: "result_filter", Type: core.ConnectionTypeString, Label: "Result", Placeholder: "blank for any result — only meaningful for completed builds", Options: []core.ConnectionOption{{Name: "Succeeded", Value: "succeeded"}, {Name: "Partially Succeeded", Value: "partiallySucceeded"}, {Name: "Failed", Value: "failed"}, {Name: "Canceled", Value: "canceled"}}},
	{Name: "branch_name", Type: core.ConnectionTypeString, Label: "Branch", Placeholder: "main — matched as a full ref (refs/heads/main)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "follow continuation tokens until every page is fetched"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "how many to return (default 50, max 1000)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Builds"},
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
	if ids := azuredevops.SplitCommaList(azuredevops.OptionalString("definition_ids", inputs)); len(ids) > 0 {
		q.Set("definitions", strings.Join(ids, ","))
	}
	if v := azuredevops.OptionalString("status_filter", inputs); v != "" {
		q.Set("statusFilter", v)
	}
	if v := azuredevops.OptionalString("result_filter", inputs); v != "" {
		q.Set("resultFilter", v)
	}
	if v := azuredevops.OptionalString("branch_name", inputs); v != "" {
		q.Set("branchName", azuredevops.FullRefName(v))
	}
	returnAll := azuredevops.ApplyPaging(q, inputs)

	items, capped, err := azuredevops.ListAll(flow, auth, azuredevops.Request{
		Method: http.MethodGet,
		Path:   azuredevops.ProjectPath(project) + "/_apis/build/builds",
		Query:  q,
	}, returnAll)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	return azuredevops.ListResult(items, azuredevops.ListSummary("build", len(items), returnAll, capped)), nil
}
