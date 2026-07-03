package scheduling_calcom_schedule_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Get Many Schedules"
	Description  = "List your Cal.com availability schedules."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+list"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Schedules"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "next_skip", Type: core.ConnectionTypeInteger, Label: "Next Skip"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	items, next, _, err := calcom.ListResources(token, "/schedules", calcom.VersionSchedules, nil, 0, 0, false)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ListResult(items, next, fmt.Sprintf("Retrieved %d schedules", len(items))), nil
}
