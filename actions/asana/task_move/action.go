package asana_task_move

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Move Task to Section"
	Description  = "Move an Asana task into a section (a column) of a project. Pick the project to load its sections."
	Website      = "https://www.flomation.co"
	Icon         = "asana+arrow-right-arrow-left"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

// project_id is a helper input: it scopes the Section picker but is not sent to
// Asana — the move is keyed on the section id.
var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Task ID", Placeholder: "The task to move", Required: true},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "The workspace that owns the project (used to load the project picker)"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "The project that owns the section (used to load the section picker)"},
	{Name: "section", Type: core.ConnectionTypeString, Label: "Section", Placeholder: "The section to move the task into", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Task ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Result"},
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
	section, err := asana.RequiredString("section", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"task": id}
	if _, err := asana.WriteObject(auth, http.MethodPost, "/sections/"+url.PathEscape(section)+"/addTask", body, url.Values{}); err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.SuccessResult(id, map[string]interface{}{"task": id, "section": section}, "Moved task "+id+" to section "+section), nil
}
