package scheduling_acuity_appointment_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Get Appointment"
	Description  = "Retrieve a single Acuity appointment by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+magnifying-glass"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "appointment_id", Type: core.ConnectionTypeInteger, Label: "Appointment ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Appointment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Appointment"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	userID, apiKey, err := acuity.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := acuity.RequiredInt("appointment_id", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}

	resp, err := acuity.GetObject(userID, apiKey, fmt.Sprintf("/appointments/%d", id), nil)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ResourceResult(resp, fmt.Sprintf("Retrieved appointment %d", id)), nil
}
