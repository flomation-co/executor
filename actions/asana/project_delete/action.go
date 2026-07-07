package asana_project_delete

import (
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Delete Project"
	Description  = "Delete an Asana project by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "asana+trash"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "The project to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Project ID"},
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
	id, err := asana.RequiredString("project_id", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	if err := asana.DeleteResource(auth, "/projects/"+url.PathEscape(id)); err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.SuccessResult(id, nil, "Deleted project "+id), nil
}
