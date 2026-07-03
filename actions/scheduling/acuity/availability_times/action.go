package scheduling_acuity_availability_times

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Get Available Times"
	Description  = "List the available start times on a given date for an Acuity appointment type."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+clock"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "appointment_type_id", Type: core.ConnectionTypeInteger, Label: "Appointment Type ID", Required: true},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Date", Placeholder: "2026-07-10 (YYYY-MM-DD)", Required: true},
	{Name: "calendar_id", Type: core.ConnectionTypeInteger, Label: "Calendar ID", Placeholder: "Limit to a calendar (optional)"},
	{Name: "timezone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "America/New_York (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Available Times"},
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
	date, err := acuity.RequiredString("date", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	q.Set("appointmentTypeID", fmt.Sprintf("%d", typeID))
	q.Set("date", date)
	acuity.AddIntFilter(q, inputs, "calendarID", "calendar_id")
	acuity.AddFilter(q, inputs, "timezone", "timezone")

	items, err := acuity.GetList(userID, apiKey, "/availability/times", q)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ListResult(items, fmt.Sprintf("Found %d available times on %s", len(items), date)), nil
}
