package scheduling_calcom_schedule_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Update Schedule"
	Description  = "Update a Cal.com availability schedule. Only supplied fields change."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+pencil"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "schedule_id", Type: core.ConnectionTypeInteger, Label: "Schedule ID", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "New name (optional)"},
	{Name: "time_zone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "New time zone (optional)"},
	{Name: "is_default", Type: core.ConnectionTypeBoolean, Label: "Set as Default"},
	{Name: "availability_json", Type: core.ConnectionTypeObject, Label: "Availability (JSON array, advanced)", Placeholder: `[{"days":["Monday"],"startTime":"09:00","endTime":"17:00"}]`},
	{Name: "overrides_json", Type: core.ConnectionTypeObject, Label: "Date Overrides (JSON array, advanced)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Schedule ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Schedule"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := calcom.RequiredInt("schedule_id", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{}
	calcom.SetIfString(body, inputs, "name", "name")
	calcom.SetIfString(body, inputs, "timeZone", "time_zone")
	calcom.SetIfBoolPresent(body, inputs, "isDefault", "is_default")
	if av, err := calcom.ParseJSONArray("availability_json", inputs); err != nil {
		return calcom.ErrorResult(err.Error()), nil
	} else if av != nil {
		body["availability"] = av
	}
	if ov, err := calcom.ParseJSONArray("overrides_json", inputs); err != nil {
		return calcom.ErrorResult(err.Error()), nil
	} else if ov != nil {
		body["overrides"] = ov
	}
	if len(body) == 0 {
		return calcom.ErrorResult("no fields to update: supply at least one field"), nil
	}

	resp, err := calcom.PatchResource(token, fmt.Sprintf("/schedules/%d", id), calcom.VersionSchedules, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Updated schedule %d", id)), nil
}
