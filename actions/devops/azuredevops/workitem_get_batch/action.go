package devops_azuredevops_workitem_get_batch

import (
	"fmt"

	core "flomation.app/automate/executor"
	azuredevops "flomation.app/automate/executor/actions/devops/azuredevops"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Azure DevOps: Get Work Items (Batch)"
	Description  = "Fetch many work items by ID in one go — paste a list of IDs and get the full items back. Azure DevOps caps a batch at 200, so longer lists are split automatically. Fields and Expand cannot be combined."
	Website      = "https://www.flomation.co"
	Icon         = "azure+layer-group"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID — optional; work items are addressable organisation-wide"},
	{Name: "work_item_ids", Type: core.ConnectionTypeString, Label: "Work Item IDs", Placeholder: "comma-separated IDs, e.g. 42,43,44", Required: true},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "comma-separated reference names, e.g. System.Title,System.State — cannot be combined with Expand"},
	{Name: "expand", Type: core.ConnectionTypeString, Label: "Expand", Placeholder: "extra detail to include — cannot be combined with Fields", Options: []core.ConnectionOption{{Name: "None", Value: "none"}, {Name: "Relations", Value: "relations"}, {Name: "Fields", Value: "fields"}, {Name: "Links", Value: "links"}, {Name: "All", Value: "all"}}},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Work Items"},
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
	ids, err := azuredevops.ParseIDList(azuredevops.OptionalString("work_item_ids", inputs), "Work Item IDs")
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	fields := azuredevops.SplitCommaList(azuredevops.OptionalString("fields", inputs))
	expand := azuredevops.OptionalString("expand", inputs)
	if len(fields) > 0 && expand != "" && expand != "none" {
		return azuredevops.ErrorResult("Fields and Expand cannot be used together — Azure DevOps rejects the pair; clear one of them"), nil
	}

	items, err := azuredevops.FetchWorkItems(flow, auth, azuredevops.OptionalString("project", inputs), ids, fields, expand)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}
	return azuredevops.ListResult(items, fmt.Sprintf("Fetched %d of %d requested work item(s)", len(items), len(ids))), nil
}
