package scheduling_calcom_slot_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Get Available Slots"
	Description  = "Fetch bookable time slots for a Cal.com event type across a date range. Returns slots grouped by day."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+clock"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "event_type_id", Type: core.ConnectionTypeInteger, Label: "Event Type ID", Required: true},
	{Name: "start", Type: core.ConnectionTypeString, Label: "Range Start", Placeholder: "2026-07-10 or 2026-07-10T00:00:00Z", Required: true},
	{Name: "end", Type: core.ConnectionTypeString, Label: "Range End", Placeholder: "2026-07-17 or 2026-07-17T00:00:00Z", Required: true},
	{Name: "time_zone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "Return slot times in this zone (optional)"},
	{Name: "duration", Type: core.ConnectionTypeInteger, Label: "Duration (minutes)", Placeholder: "For dynamic/variable-length events (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Slots by Day"},
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
	start, err := calcom.RequiredString("start", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	end, err := calcom.RequiredString("end", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	q.Set("eventTypeId", fmt.Sprintf("%d", id))
	q.Set("start", start)
	q.Set("end", end)
	calcom.AddFilter(q, inputs, "timeZone", "time_zone")
	if d, ok := calcom.OptionalInt("duration", inputs); ok {
		q.Set("duration", fmt.Sprintf("%d", d))
	}

	// The slots endpoint returns data as an object keyed by date, not a
	// paginated array, so decode it as a single resource.
	resp, err := calcom.GetResource(token, "/slots", calcom.VersionSlots, q)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return map[string]interface{}{
		"result":      resp,
		"tool_result": fmt.Sprintf("Retrieved available slots for event type %d (%s to %s)", id, start, end),
		"success":     true,
		"error":       "",
	}, nil
}
