package scheduling_acuity_appointment_payments

import (
	"fmt"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Get Appointment Payments"
	Description  = "List the payment transactions recorded against an Acuity appointment."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+dollar-sign"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "appointment_id", Type: core.ConnectionTypeInteger, Label: "Appointment ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Payments"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	items, err := acuity.GetList(userID, apiKey, fmt.Sprintf("/appointments/%d/payments", id), nil)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ListResult(items, fmt.Sprintf("Retrieved %d payments for appointment %d", len(items), id)), nil
}
