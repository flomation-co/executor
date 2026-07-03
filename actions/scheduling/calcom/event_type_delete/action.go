package scheduling_calcom_event_type_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Delete Event Type"
	Description  = "Permanently delete a Cal.com event type."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+trash"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "event_type_id", Type: core.ConnectionTypeInteger, Label: "Event Type ID", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, ok := calcom.OptionalInt("event_type_id", inputs)
	if !ok {
		return calcom.ErrorResult("event_type_id is required"), nil
	}

	if err := calcom.DeleteResource(token, fmt.Sprintf("/event-types/%d", id), calcom.VersionEventTypes); err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted event type %d", id),
		"success":     true,
		"error":       "",
	}, nil
}
