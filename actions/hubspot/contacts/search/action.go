package hubspot_contacts_search

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Search Contacts"
	Description  = "Search HubSpot contacts by free text or a property filter (e.g. email EQ jane@example.com). Returns matching contacts."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+search"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "query", Type: core.ConnectionTypeString, Label: "Query", Placeholder: "Free-text search (optional)"},
	{Name: "filter_property", Type: core.ConnectionTypeString, Label: "Filter Property", Placeholder: "email"},
	{Name: "filter_operator", Type: core.ConnectionTypeString, Label: "Filter Operator", Placeholder: "EQ", Options: []core.ConnectionOption{
		{Name: "Equals", Value: "EQ"},
		{Name: "Not equals", Value: "NEQ"},
		{Name: "Greater than", Value: "GT"},
		{Name: "Greater than or equal", Value: "GTE"},
		{Name: "Less than", Value: "LT"},
		{Name: "Less than or equal", Value: "LTE"},
		{Name: "Contains token", Value: "CONTAINS_TOKEN"},
		{Name: "Has property", Value: "HAS_PROPERTY"},
		{Name: "Has no property", Value: "NOT_HAS_PROPERTY"},
	}},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "jane@example.com"},
	{Name: "filter_groups", Type: core.ConnectionTypeObject, Label: "Filter Groups (advanced)", Placeholder: "Raw HubSpot filterGroups array; overrides the simple filter"},
	{Name: "properties", Type: core.ConnectionTypeString, Label: "Properties", Placeholder: "Comma-separated property names to return (optional)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "100"},
	{Name: "after", Type: core.ConnectionTypeString, Label: "After (cursor)", Placeholder: "Pagination cursor from a previous page"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Contacts"},
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

	body := hubspot.BuildSearchBody(inputs)
	resp, err := hubspot.SearchObjects(apiKey, "contacts", body)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	out := hubspot.ListResult(resp, "")
	out["tool_result"] = fmt.Sprintf("Found %v contacts", out["count"])
	return out, nil
}
