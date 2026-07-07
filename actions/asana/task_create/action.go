package asana_task_create

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Task"
	Description  = "Create a task in Asana. Give it a name and workspace, then optionally assign it, set a due date, notes, and the projects it belongs to."
	Website      = "https://www.flomation.co"
	Icon         = "asana+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "The workspace to create the task in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The name of the task", Required: true},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes", Placeholder: "A description for the task"},
	{Name: "assignee", Type: core.ConnectionTypeString, Label: "Assignee", Placeholder: "The user to assign the task to"},
	{Name: "due_on", Type: core.ConnectionTypeString, Label: "Due On", Placeholder: "Due date, e.g. 2026-07-31 (YYYY-MM-DD)"},
	{Name: "projects", Type: core.ConnectionTypeString, Label: "Projects", Placeholder: "Comma-separated project IDs to add the task to"},
	{Name: "followers", Type: core.ConnectionTypeString, Label: "Followers", Placeholder: "Comma-separated user IDs to follow the task"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "Extra Asana fields as JSON, e.g. {\"start_on\":\"2026-07-01\",\"html_notes\":\"<body>..</body>\"}"},
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
	workspace, err := asana.RequiredString("workspace", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	name, err := asana.RequiredString("name", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"workspace": workspace, "name": name}
	asana.SetIfPresent(body, inputs, "notes", "notes")
	asana.SetIfPresent(body, inputs, "assignee", "assignee")
	asana.SetIfPresent(body, inputs, "due_on", "due_on")
	asana.SetStringListIfPresent(body, inputs, "projects", "projects")
	asana.SetStringListIfPresent(body, inputs, "followers", "followers")
	if err := asana.MergeAdditionalFields(body, inputs); err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	obj, err := asana.WriteObject(auth, http.MethodPost, "/tasks", body, url.Values{})
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ResourceResult(obj, fmt.Sprintf("Created task %q", name)), nil
}
