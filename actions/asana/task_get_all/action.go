package asana_task_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Tasks"
	Description  = "List Asana tasks. Narrow by project, or by assignee within a workspace, or by section."
	Website      = "https://www.flomation.co"
	Icon         = "asana+list"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "project", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "List tasks in this project"},
	{Name: "section", Type: core.ConnectionTypeString, Label: "Section", Placeholder: "List tasks in this section"},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "Required when filtering by Assignee"},
	{Name: "assignee", Type: core.ConnectionTypeString, Label: "Assignee", Placeholder: "List tasks assigned to this user (needs Workspace)"},
	{Name: "opt_fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. name,completed,due_on"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every task (ignores Limit)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum number of tasks to return (1-100)"},
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
	q := url.Values{}
	asana.SetOptFields(q, inputs, "opt_fields")
	if v := asana.OptionalString("project", inputs); v != "" {
		q.Set("project", v)
	}
	if v := asana.OptionalString("section", inputs); v != "" {
		q.Set("section", v)
	}
	if v := asana.OptionalString("workspace", inputs); v != "" {
		q.Set("workspace", v)
	}
	if v := asana.OptionalString("assignee", inputs); v != "" {
		q.Set("assignee", v)
	}
	if q.Get("project") == "" && q.Get("section") == "" && q.Get("assignee") == "" {
		return asana.ErrorResult("provide a Project, a Section, or an Assignee (with Workspace) to list tasks"), nil
	}
	// Asana's GET /tasks requires a workspace whenever filtering by assignee.
	if q.Get("assignee") != "" && q.Get("workspace") == "" {
		return asana.ErrorResult("Workspace is required when filtering by Assignee"), nil
	}
	returnAll, _ := asana.OptionalBoolSet("return_all", inputs)
	limit, _ := asana.OptionalInt("limit", inputs)
	items, err := asana.ListAll(auth, "/tasks", q, limit, returnAll)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ListResult(items, fmt.Sprintf("Retrieved %d task(s)", len(items))), nil
}
