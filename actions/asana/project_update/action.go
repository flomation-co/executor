package asana_project_update

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Update Project"
	Description  = "Change an existing Asana project — rename it, edit notes, set the owner, colour or due date."
	Website      = "https://www.flomation.co"
	Icon         = "asana+pen"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "The project to update", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A new name for the project"},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes", Placeholder: "A new description for the project"},
	{Name: "owner", Type: core.ConnectionTypeString, Label: "Owner", Placeholder: "Set the project owner (a user ID)"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "Extra Asana fields as JSON, e.g. {\"color\":\"light-blue\",\"archived\":true}"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Project ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Project"},
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
	body := map[string]interface{}{}
	asana.SetIfPresent(body, inputs, "name", "name")
	asana.SetIfPresent(body, inputs, "notes", "notes")
	asana.SetIfPresent(body, inputs, "owner", "owner")
	if err := asana.MergeAdditionalFields(body, inputs); err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	obj, err := asana.WriteObject(auth, http.MethodPut, "/projects/"+url.PathEscape(id), body, url.Values{})
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ResourceResult(obj, "Updated project "+id), nil
}
