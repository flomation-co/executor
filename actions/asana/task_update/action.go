package asana_task_update

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Task"
	Description  = "Change an existing Asana task — rename it, edit notes, reassign it, set a due date, or mark it complete."
	Website      = "https://www.flomation.co"
	Icon         = "asana+pen"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Task ID", Placeholder: "The ID of the task to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A new name for the task"},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes", Placeholder: "A new description for the task"},
	{Name: "assignee", Type: core.ConnectionTypeString, Label: "Assignee", Placeholder: "Reassign the task to this user"},
	{Name: "due_on", Type: core.ConnectionTypeString, Label: "Due On", Placeholder: "Due date, e.g. 2026-07-31 (YYYY-MM-DD)"},
	{Name: "completed", Type: core.ConnectionTypeBoolean, Label: "Completed", Placeholder: "Mark the task complete or incomplete"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "Extra Asana fields as JSON"},
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
	body := map[string]interface{}{}
	asana.SetIfPresent(body, inputs, "name", "name")
	asana.SetIfPresent(body, inputs, "notes", "notes")
	asana.SetIfPresent(body, inputs, "assignee", "assignee")
	asana.SetIfPresent(body, inputs, "due_on", "due_on")
	asana.SetBoolIfSet(body, inputs, "completed", "completed")
	if err := asana.MergeAdditionalFields(body, inputs); err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	// Asana's PUT is a PARTIAL update: only the fields present in the data body
	// are changed; omitted fields are left untouched (verified empirically — it is
	// not a full replace, so this can't null out unspecified fields).
	obj, err := asana.WriteObject(auth, http.MethodPut, "/tasks/"+url.PathEscape(id), body, url.Values{})
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ResourceResult(obj, "Updated task "+id), nil
}
