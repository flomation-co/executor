package hubspot_deal_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	hubspot "flomation.app/automate/executor/actions/hubspot"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Deal: Update"
	Description  = "Update properties on an existing HubSpot deal. Only the fields you set are changed."
	Website      = "https://www.flomation.co"
	Icon         = "hubspot+pencil"
	Date         = "30/06/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "HubSpot Private App Token", Placeholder: "pat-...", Required: true},
	{Name: "deal_id", Type: core.ConnectionTypeString, Label: "Deal ID", Placeholder: "12345", Required: true},
	{Name: "dealname", Type: core.ConnectionTypeString, Label: "Deal Name", Placeholder: "Acme - Q3 expansion"},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Amount", Placeholder: "5000"},
	{Name: "pipeline", Type: core.ConnectionTypeString, Label: "Pipeline", Placeholder: "default"},
	{Name: "dealstage", Type: core.ConnectionTypeString, Label: "Deal Stage", Placeholder: "closedwon"},
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

	id, err := hubspot.RequiredString("deal_id", inputs)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	props := hubspot.BuildProperties(inputs, "dealname", "amount", "pipeline", "dealstage", "closedate", "hubspot_owner_id")
	if len(props) == 0 {
		return hubspot.ErrorResult("at least one property to update is required"), nil
	}

	obj, err := hubspot.UpdateObject(apiKey, "deals", id, props)
	if err != nil {
		return hubspot.ErrorResult(err.Error()), nil
	}

	return hubspot.ObjectResult(obj, fmt.Sprintf("Updated deal %s", id)), nil
}
