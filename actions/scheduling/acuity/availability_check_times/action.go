package scheduling_acuity_availability_check_times

import (
	"fmt"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Check Time Availability"
	Description  = "Validate whether a specific date/time is bookable for an Acuity appointment type before booking it."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+circle-check"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "appointment_type_id", Type: core.ConnectionTypeInteger, Label: "Appointment Type ID", Required: true},
	{Name: "datetime", Type: core.ConnectionTypeString, Label: "Date & Time", Placeholder: "2026-07-10T09:00:00-0700 (ISO 8601)", Required: true},
	{Name: "calendar_id", Type: core.ConnectionTypeInteger, Label: "Calendar ID", Placeholder: "Check against a calendar (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Check Results"},
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
	typeID, err := acuity.RequiredInt("appointment_type_id", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	datetime, err := acuity.RequiredString("datetime", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}

	// check-times takes an array of slots to validate; build a single-slot
	// request from the inputs.
	slot := map[string]interface{}{
		"appointmentTypeID": typeID,
		"datetime":          datetime,
	}
	if calID, ok := acuity.OptionalInt("calendar_id", inputs); ok {
		slot["calendarID"] = calID
	}

	items, err := acuity.PostList(userID, apiKey, "/availability/check-times", []interface{}{slot})
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ListResult(items, fmt.Sprintf("Checked availability for %s", datetime)), nil
}
