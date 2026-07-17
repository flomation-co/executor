package devops_azuredevops_workitem_get

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
	Name         = "Azure DevOps: Get Work Item"
	Description  = "Get a work item by ID. Expand pulls in relations, links or every field; Fields narrows the response to the reference names you list. The two cannot be combined — Azure DevOps rejects that pairing."
	Website      = "https://www.flomation.co"
	Icon         = "azure+eye"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "organisation_url", Type: core.ConnectionTypeString, Label: "Organisation URL", Placeholder: "https://dev.azure.com/your-org (or https://your-org.visualstudio.com)", Required: true},
	{Name: "personal_access_token", Type: core.ConnectionTypeSecret, Label: "Personal Access Token", Placeholder: "User settings ▸ Personal access tokens — the token value, shown once at creation", Required: true},
	{Name: "api_version", Type: core.ConnectionTypeString, Label: "API Version", Placeholder: "7.1 — leave blank unless Microsoft asks you to pin another version"},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "project name or ID — optional; work items are addressable organisation-wide"},
	{Name: "work_item_id", Type: core.ConnectionTypeInteger, Label: "Work Item", Placeholder: "the work item ID, e.g. 42", Required: true},
	{Name: "expand", Type: core.ConnectionTypeString, Label: "Expand", Placeholder: "extra detail to include — cannot be combined with Fields", Options: []core.ConnectionOption{{Name: "None", Value: "none"}, {Name: "Relations", Value: "relations"}, {Name: "Fields", Value: "fields"}, {Name: "Links", Value: "links"}, {Name: "All", Value: "all"}}},
	{Name: "fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "comma-separated reference names, e.g. System.Title,System.State — blank for the default set"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Work Item ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Work Item"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Work Item URL"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := azuredevops.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	itemID, err := azuredevops.RequiredInt("work_item_id", "Work Item", inputs)
	if err != nil {
		return azuredevops.ErrorResult(err.Error()), nil
	}

	expand := azuredevops.OptionalString("expand", inputs)
	fields := azuredevops.SplitCommaList(azuredevops.OptionalString("fields", inputs))
	if len(fields) > 0 && expand != "" && expand != "none" {
		return azuredevops.ErrorResult("Fields and Expand cannot be used together — Azure DevOps rejects the pair; clear one of them"), nil
	}

	q := url.Values{}
	if len(fields) > 0 {
		q.Set("fields", strings.Join(fields, ","))
	} else if expand != "" && expand != "none" {
		q.Set("$expand", expand)
	}

	// The project segment is optional: work items are addressable
	// organisation-wide by ID, so a flow holding only an ID still works.
	path := "/_apis/wit/workitems/" + strconv.Itoa(itemID)
	if project := azuredevops.OptionalString("project", inputs); project != "" {
		path = azuredevops.ProjectPath(project) + path
	}

	resp, err := azuredevops.Do(flow, auth, azuredevops.Request{Method: http.MethodGet, Path: path, Query: q})
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
	out := azuredevops.ResourceResult(obj, fmt.Sprintf("Fetched work item %d", itemID))
	out["url"], _ = obj["url"].(string)
	return out, nil
}
