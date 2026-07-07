package asana_subtask_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Subtasks"
	Description  = "List the subtasks of an Asana task."
	Website      = "https://www.flomation.co"
	Icon         = "asana+list"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "task_id", Type: core.ConnectionTypeString, Label: "Parent Task ID", Placeholder: "The task whose subtasks to fetch", Required: true},
	{Name: "opt_fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. name,completed"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every subtask (ignores Limit)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum number of subtasks to return (1-100)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Subtasks"},
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
	taskID, err := asana.RequiredString("task_id", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	q := url.Values{}
	asana.SetOptFields(q, inputs, "opt_fields")
	returnAll, _ := asana.OptionalBoolSet("return_all", inputs)
	limit, _ := asana.OptionalInt("limit", inputs)
	items, err := asana.ListAll(auth, "/tasks/"+url.PathEscape(taskID)+"/subtasks", q, limit, returnAll)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ListResult(items, fmt.Sprintf("Retrieved %d subtask(s)", len(items))), nil
}
