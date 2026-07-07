package asana_task_get

import (
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Task"
	Description  = "Fetch a single Asana task by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "asana+magnifying-glass"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Task ID", Placeholder: "The ID of the task to fetch", Required: true},
	{Name: "opt_fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. name,notes,assignee,due_on"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Task ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Task"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := asana.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := asana.RequiredString("id", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	q := url.Values{}
	asana.SetOptFields(q, inputs, "opt_fields")
	obj, err := asana.GetObject(auth, "/tasks/"+url.PathEscape(id), q)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ResourceResult(obj, "Retrieved task "+id), nil
}
