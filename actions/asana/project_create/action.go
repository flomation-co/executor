package asana_project_create

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
	Name         = "Create Project"
	Description  = "Create a project in an Asana workspace. In an organization, also choose a team."
	Website      = "https://www.flomation.co"
	Icon         = "asana+plus"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "The workspace to create the project in", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "The name of the project", Required: true},
	{Name: "team", Type: core.ConnectionTypeString, Label: "Team", Placeholder: "The team to own the project (required in an organization)"},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes", Placeholder: "A description for the project"},
	{Name: "additional_fields", Type: core.ConnectionTypeObject, Label: "Additional Fields", Placeholder: "Extra Asana fields as JSON, e.g. {\"color\":\"dark-green\",\"due_on\":\"2026-08-01\"}"},
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
	workspace, err := asana.RequiredString("workspace", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	name, err := asana.RequiredString("name", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	body := map[string]interface{}{"workspace": workspace, "name": name}
	asana.SetIfPresent(body, inputs, "team", "team")
	asana.SetIfPresent(body, inputs, "notes", "notes")
	if err := asana.MergeAdditionalFields(body, inputs); err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	obj, err := asana.WriteObject(auth, http.MethodPost, "/projects", body, url.Values{})
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ResourceResult(obj, fmt.Sprintf("Created project %q", name)), nil
}
