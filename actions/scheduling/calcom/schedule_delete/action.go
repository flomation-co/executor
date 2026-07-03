package scheduling_calcom_schedule_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Delete Schedule"
	Description  = "Permanently delete a Cal.com availability schedule."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+trash"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "schedule_id", Type: core.ConnectionTypeInteger, Label: "Schedule ID", Required: true},
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
	id, ok := calcom.OptionalInt("schedule_id", inputs)
	if !ok {
		return calcom.ErrorResult("schedule_id is required"), nil
	}

	if err := calcom.DeleteResource(token, fmt.Sprintf("/schedules/%d", id), calcom.VersionSchedules); err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted schedule %d", id),
		"success":     true,
		"error":       "",
	}, nil
}
