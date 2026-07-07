package asana_project_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Projects"
	Description  = "List projects in an Asana workspace, optionally filtered by team or archived state."
	Website      = "https://www.flomation.co"
	Icon         = "asana+list"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "The workspace whose projects to list", Required: true},
	{Name: "team", Type: core.ConnectionTypeString, Label: "Team", Placeholder: "Only projects owned by this team"},
	{Name: "archived", Type: core.ConnectionTypeBoolean, Label: "Archived", Placeholder: "Only archived (on) or only unarchived (off) projects"},
	{Name: "opt_fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. name,archived"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every project (ignores Limit)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum number of projects to return (1-100)"},
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
	workspace, err := asana.RequiredString("workspace", inputs)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	q := url.Values{}
	q.Set("workspace", workspace)
	asana.SetOptFields(q, inputs, "opt_fields")
	if v := asana.OptionalString("team", inputs); v != "" {
		q.Set("team", v)
	}
	if v, ok := asana.OptionalBoolSet("archived", inputs); ok {
		if v {
			q.Set("archived", "true")
		} else {
			q.Set("archived", "false")
		}
	}
	returnAll, _ := asana.OptionalBoolSet("return_all", inputs)
	limit, _ := asana.OptionalInt("limit", inputs)
	items, err := asana.ListAll(auth, "/projects", q, limit, returnAll)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ListResult(items, fmt.Sprintf("Retrieved %d project(s)", len(items))), nil
}
