package hubspot_deals_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "List Deals"
	Description  = "List HubSpot deals a page at a time. Pass the returned After cursor to fetch the next page."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+list"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "100 (max)"},
	{Name: "after", Type: core.ConnectionTypeString, Label: "After (cursor)", Placeholder: "Pagination cursor from a previous page"},
	{Name: "properties", Type: core.ConnectionTypeString, Label: "Properties", Placeholder: "Comma-separated property names (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Deals"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "after", Type: core.ConnectionTypeString, Label: "Next Cursor"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := hubspot.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	limit, _ := hubspot.OptionalInt("limit", inputs)
	after := hubspot.OptionalString("after", inputs)
	props := hubspot.CSVToList(hubspot.OptionalString("properties", inputs))

	resp, err := hubspot.ListObjects(apiKey, "deals", limit, after, props)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	out := hubspot.ListResult(resp, "")
	out["tool_result"] = fmt.Sprintf("Listed %v deals", out["count"])
	return out, nil
}
