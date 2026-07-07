package asana_tag_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	asana "flomation.app/automate/executor/actions/asana"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Tags"
	Description  = "List the tags in an Asana workspace."
	Website      = "https://www.flomation.co"
	Icon         = "asana+list"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Asana Personal Access Token", Required: true},
	{Name: "workspace", Type: core.ConnectionTypeString, Label: "Workspace", Placeholder: "The workspace whose tags to fetch", Required: true},
	{Name: "opt_fields", Type: core.ConnectionTypeString, Label: "Fields", Placeholder: "Comma-separated fields to return, e.g. name,color"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every tag (ignores Limit)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum number of tags to return (1-100)"},
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
	returnAll, _ := asana.OptionalBoolSet("return_all", inputs)
	limit, _ := asana.OptionalInt("limit", inputs)
	items, err := asana.ListAll(auth, "/tags", q, limit, returnAll)
	if err != nil {
		return asana.ErrorResult(err.Error()), nil
	}
	return asana.ListResult(items, fmt.Sprintf("Retrieved %d tag(s)", len(items))), nil
}
