package helpdesk_intercom_ticket_search

import (
	"fmt"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Search Tickets"
	Description  = "Find Intercom tickets matching a filter — for example every ticket in a given state, assigned to a teammate, or created after a date. Use the advanced JSON query for AND/OR combinations."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+magnifying-glass"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Intercom access token (Developer Hub → Authentication)", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
	{Name: "field", Type: core.ConnectionTypeString, Label: "Field", Placeholder: "e.g. state — also ticket_type_id, admin_assignee_id, created_at, or open"},
	{
		Name:  "operator",
		Type:  core.ConnectionTypeString,
		Label: "Operator",
		Options: []core.ConnectionOption{
			{Name: "Equals", Value: "="},
			{Name: "Not equals", Value: "!="},
			{Name: "Is any of (comma-separated)", Value: "IN"},
			{Name: "Is none of (comma-separated)", Value: "NIN"},
			{Name: "Greater than", Value: ">"},
			{Name: "Less than", Value: "<"},
			{Name: "Contains", Value: "~"},
			{Name: "Doesn't contain", Value: "!~"},
			{Name: "Starts with", Value: "^"},
			{Name: "Ends with", Value: "$"},
		},
	},
	{Name: "value", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "e.g. in_progress — comma-separate several values for the any of/none of operators"},
	{Name: "query_json", Type: core.ConnectionTypeObject, Label: "Advanced Query (JSON)", Placeholder: `A full Intercom search query for AND/OR combinations — overrides Field/Operator/Value, e.g. {"operator":"AND","value":[…]}`},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results (default 50)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Fetch every match (ignores Limit)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Tickets"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	query, err := intercom.BuildSearchQuery(inputs)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}

	limit, _ := intercom.OptionalInt("limit", inputs)
	returnAll, _ := intercom.OptionalBoolSet("return_all", inputs)
	items, err := intercom.SearchAll(auth, "/tickets/search", query, "tickets", limit, returnAll)
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	return intercom.ListResult(items, fmt.Sprintf("Found %d ticket(s)", len(items))), nil
}
