package scheduling_calcom_event_type_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Get Event Type"
	Description  = "Retrieve a single Cal.com event type by its ID."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+magnifying-glass"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "event_type_id", Type: core.ConnectionTypeInteger, Label: "Event Type ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Event Type ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Event Type"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := calcom.RequiredInt("event_type_id", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	resp, err := calcom.GetResource(token, fmt.Sprintf("/event-types/%d", id), calcom.VersionEventTypes, nil)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Retrieved event type %d", id)), nil
}
