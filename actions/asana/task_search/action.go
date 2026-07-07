package asana_task_search

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Search Tasks"
	Description  = "Search for Asana tasks in a workspace by text in their name or notes."
	Website      = "https://www.flomation.co"
	Icon         = "asana+magnifying-glass"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "The workspace to search in", Required: true},
	{Name: "text", Type: core.ConnectionTypeString, Label: "Text", Placeholder: "Text to search for in task names and notes"},
	{Name: "opt_fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. name,completed,assignee"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "Extra Asana search filters as JSON, e.g. {\"completed\":false}"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Tasks"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := asana.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	workspace, err := asana.RequiredString("workspace", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	q := url.Values{}
	asana.SetOptFields(q, inputs, "opt_fields")
	if v := asana.OptionalString("text", inputs); v != "" {
		q.Set("text", v)
	}
	// Search filters (completed, assignee.any, etc.) can be passed as raw query
	// params via additional_fields — the search endpoint has a large filter set.
	if raw, err := asana.OptionalJSON("additional_fields", inputs); err != nil {
		return asana.ErrorResult(err.Error()), nil
	} else if obj, ok := raw.(map[string]interface{}); ok {
		for k, val := range obj {
			q.Set(k, fmt.Sprintf("%v", val))
		}
	}
	items, err := asana.ListPageOnly(auth, "/workspaces/"+url.PathEscape(workspace)+"/tasks/search", q)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ListResult(items, fmt.Sprintf("Found %d task(s)", len(items))), nil
}
