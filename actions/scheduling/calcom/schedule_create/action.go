package scheduling_calcom_schedule_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Create Schedule"
	Description  = "Create a Cal.com availability schedule defining the hours you can be booked."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+plus"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "e.g. Working hours", Required: true},
	{Name: "time_zone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "Europe/London", Required: true},
	{Name: "is_default", Type: core.ConnectionTypeBoolean, Label: "Set as Default"},
	{Name: "availability_json", Type: core.ConnectionTypeObject, Label: "Availability (JSON array, advanced)", Placeholder: `[{"days":["Monday","Tuesday"],"startTime":"09:00","endTime":"17:00"}]`},
	{Name: "overrides_json", Type: core.ConnectionTypeObject, Label: "Date Overrides (JSON array, advanced)", Placeholder: `[{"date":"2026-12-25","startTime":"00:00","endTime":"00:00"}]`},
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
	name, err := calcom.RequiredString("name", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	tz, err := calcom.RequiredString("time_zone", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"name":      name,
		"timeZone":  tz,
		"isDefault": calcom.OptionalBool("is_default", inputs),
	}
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

	resp, err := calcom.PostResource(token, "/schedules", calcom.VersionSchedules, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Created schedule %q", name)), nil
}
