package asana_section_create

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Section"
	Description  = "Create a section (a column) in an Asana project."
	Website      = "https://www.flomation.co"
	Icon         = "asana+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "Optional — used only to load the Project picker (not sent to Asana)"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "The project to add the section to", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The name of the section", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Section ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Section"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := asana.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	projectID, err := asana.RequiredString("project_id", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	name, err := asana.RequiredString("name", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"name": name}
	obj, err := asana.WriteObject(auth, http.MethodPost, "/projects/"+url.PathEscape(projectID)+"/sections", body, url.Values{})
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ResourceResult(obj, "Created section "+name), nil
}
