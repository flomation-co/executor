package scheduling_acuity_appointment_get_all

import (
	"fmt"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Get Many Appointments"
	Description  = "List Acuity appointments, filtered by date range, calendar, type, client details or cancellation state."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+list"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "min_date", Type: core.ConnectionTypeString, Label: "Start Date", Placeholder: "2026-07-01 (optional)"},
	{Name: "max_date", Type: core.ConnectionTypeString, Label: "End Date", Placeholder: "2026-07-31 (optional)"},
	{Name: "calendar_id", Type: core.ConnectionTypeInteger, Label: "Calendar ID", Placeholder: "Filter to a calendar (optional)"},
	{Name: "appointment_type_id", Type: core.ConnectionTypeInteger, Label: "Appointment Type ID", Placeholder: "Filter to a type (optional)"},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Placeholder: "Filter by client first name (optional)"},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Placeholder: "Filter by client last name (optional)"},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "Filter by client email (optional)"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "Filter by client phone (optional)"},
	{Name: "canceled", Type: core.ConnectionTypeBoolean, Label: "Include Canceled", Placeholder: "Return canceled appointments"},
	{Name: "direction", Type: core.ConnectionTypeString, Label: "Sort Direction", Options: []core.ConnectionOption{
		{Name: "Newest First", Value: "DESC"},
		{Name: "Oldest First", Value: "ASC"},
	}},
	{Name: "max", Type: core.ConnectionTypeInteger, Label: "Max Results", Placeholder: "1-1000, default 100"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Appointments"},
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

	q := url.Values{}
	acuity.AddFilter(q, inputs, "minDate", "min_date")
	acuity.AddFilter(q, inputs, "maxDate", "max_date")
	acuity.AddIntFilter(q, inputs, "calendarID", "calendar_id")
	acuity.AddIntFilter(q, inputs, "appointmentTypeID", "appointment_type_id")
	acuity.AddFilter(q, inputs, "firstName", "first_name")
	acuity.AddFilter(q, inputs, "lastName", "last_name")
	acuity.AddFilter(q, inputs, "email", "email")
	acuity.AddFilter(q, inputs, "phone", "phone")
	acuity.AddFilter(q, inputs, "direction", "direction")
	if acuity.OptionalBool("canceled", inputs) {
		q.Set("canceled", "true")
	}
	max, set := acuity.OptionalInt("max", inputs)
	q.Set("max", strconv.Itoa(acuity.ClampMax(max, set)))

	items, err := acuity.GetList(userID, apiKey, "/appointments", q)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ListResult(items, fmt.Sprintf("Retrieved %d appointments", len(items))), nil
}
