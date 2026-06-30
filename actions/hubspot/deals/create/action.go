package hubspot_deals_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Create Deal"
	Description  = "Create a new deal in HubSpot. Set common fields directly or add any other property via Additional Properties. Returns the deal ID."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+plus"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "dealname", Type: core.ConnectionTypeString, Label: "Deal Name", Placeholder: "Acme - Q3 expansion"},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Amount", Placeholder: "5000"},
	{Name: "pipeline", Type: core.ConnectionTypeString, Label: "Pipeline", Placeholder: "default"},
	{Name: "dealstage", Type: core.ConnectionTypeString, Label: "Deal Stage", Placeholder: "appointmentscheduled"},
	{Name: "closedate", Type: core.ConnectionTypeString, Label: "Close Date", Placeholder: "2026-09-30 or epoch ms"},
	{Name: "hubspot_owner_id", Type: core.ConnectionTypeString, Label: "Owner ID", Placeholder: "HubSpot owner ID (optional)"},
	{Name: "additional_properties", Type: core.ConnectionTypeKeyValueArray, Label: "Additional Properties", Placeholder: "Any other deal property (key = HubSpot internal name)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deal ID"},
	{Name: "properties", Type: core.ConnectionTypeObject, Label: "Properties"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Deal"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := hubspot.GetAPIKey(inputs)
	if err != nil {
		return nil, err
	}

	props := hubspot.BuildProperties(inputs, "dealname", "amount", "pipeline", "dealstage", "closedate", "hubspot_owner_id")
	if len(props) == 0 {
		return hubspot.ErrorResult("at least one deal property is required"), nil
	}

	obj, err := hubspot.CreateObject(apiKey, "deals", props, nil)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	id, _ := obj["id"].(string)
	return hubspot.ObjectResult(obj, fmt.Sprintf("Created deal %s", id)), nil
}
