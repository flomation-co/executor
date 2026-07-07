package asana_subtask_create

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Subtask"
	Description  = "Add a subtask to an Asana task."
	Website      = "https://www.flomation.co"
	Icon         = "asana+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "task_id", Type: core.ConnectionTypeString, Label: "Parent Task ID", Placeholder: "The task to add the subtask to", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The name of the subtask", Required: true},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes", Placeholder: "A description for the subtask"},
	{Name: "assignee", Type: core.ConnectionTypeString, Label: "Assignee", Placeholder: "The user to assign the subtask to"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "Extra Asana fields as JSON"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Subtask ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Subtask"},
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
	name, err := asana.RequiredString("name", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"name": name}
	asana.SetIfPresent(body, inputs, "notes", "notes")
	asana.SetIfPresent(body, inputs, "assignee", "assignee")
	if err := asana.MergeAdditionalFields(body, inputs); err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	obj, err := asana.WriteObject(auth, http.MethodPost, "/tasks/"+url.PathEscape(taskID)+"/subtasks", body, url.Values{})
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ResourceResult(obj, "Created subtask "+name), nil
}
