package asana_section_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Sections"
	Description  = "List the sections (columns) of an Asana project."
	Website      = "https://www.flomation.co"
	Icon         = "asana+list"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "The workspace that owns the project (used to load the project picker)"},
	{Name: "project_id", Type: core.ConnectionTypeString, Label: "Project", Placeholder: "The project whose sections to fetch", Required: true},
	{Name: "opt_fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. name"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every section (ignores Limit)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum number of sections to return (1-100)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Results"},
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
	projectID, err := asana.RequiredString("project_id", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	q := url.Values{}
	asana.SetOptFields(q, inputs, "opt_fields")
	returnAll, _ := asana.OptionalBoolSet("return_all", inputs)
	limit, _ := asana.OptionalInt("limit", inputs)
	items, err := asana.ListAll(auth, "/projects/"+url.PathEscape(projectID)+"/sections", q, limit, returnAll)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ListResult(items, fmt.Sprintf("Retrieved %d section(s)", len(items))), nil
}
