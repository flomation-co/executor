package hubspot_ticket_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Ticket: Get"
	Description  = "Retrieve a HubSpot ticket by its ID. Optionally request specific properties and associated object types."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+search"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "ticket_id", Type: core.ConnectionTypeString, Label: "Ticket ID", Placeholder: "12345", Required: true},
	{Name: "properties", Type: core.ConnectionTypeString, Label: "Properties", Placeholder: "Comma-separated property names (optional)"},
	{Name: "associations", Type: core.ConnectionTypeString, Label: "Associations", Placeholder: "Comma-separated object types, e.g. contacts,companies (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Ticket ID"},
	{Name: "properties", Type: core.ConnectionTypeObject, Label: "Properties"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Ticket"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := hubspot.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	id, err := hubspot.RequiredString("ticket_id", inputs)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	props := hubspot.CSVToList(hubspot.OptionalString("properties", inputs))
	assoc := hubspot.CSVToList(hubspot.OptionalString("associations", inputs))

	obj, err := hubspot.GetObject(apiKey, "tickets", id, props, assoc)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	return hubspot.ObjectResult(obj, fmt.Sprintf("Retrieved ticket %s", id)), nil
}
