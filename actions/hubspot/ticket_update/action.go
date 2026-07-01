package hubspot_ticket_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Ticket: Update"
	Description  = "Update properties on an existing HubSpot ticket. Only the fields you set are changed."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+pencil"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "ticket_id", Type: core.ConnectionTypeString, Label: "Ticket ID", Placeholder: "12345", Required: true},
	{Name: "subject", Type: core.ConnectionTypeString, Label: "Subject", Placeholder: "Cannot log in"},
	{Name: "content", Type: core.ConnectionTypeText, Label: "Content", Placeholder: "Description of the issue"},
	{Name: "hs_pipeline", Type: core.ConnectionTypeString, Label: "Pipeline", Placeholder: "0 (Support Pipeline)"},
	{Name: "hs_pipeline_stage", Type: core.ConnectionTypeString, Label: "Pipeline Stage", Placeholder: "4 (Closed)"},
	{Name: "hs_ticket_priority", Type: core.ConnectionTypeString, Label: "Priority", Placeholder: "HIGH", Options: []core.ConnectionOption{
		{Name: "Low", Value: "LOW"},
		{Name: "Medium", Value: "MEDIUM"},
		{Name: "High", Value: "HIGH"},
	}},
	{Name: "additional_properties", Type: core.ConnectionTypeKeyValueArray, Label: "Additional Properties", Placeholder: "Any other ticket property (key = HubSpot internal name)"},
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

	props := hubspot.BuildProperties(inputs, "subject", "content", "hs_pipeline", "hs_pipeline_stage", "hs_ticket_priority")
	if len(props) == 0 {
		return hubspot.ErrorResult("at least one property to update is required"), nil
	}

	obj, err := hubspot.UpdateObject(apiKey, "tickets", id, props)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	return hubspot.ObjectResult(obj, fmt.Sprintf("Updated ticket %s", id)), nil
}
