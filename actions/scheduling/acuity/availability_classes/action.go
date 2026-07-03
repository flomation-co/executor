package scheduling_acuity_availability_classes

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Get Available Classes"
	Description  = "List the class offerings (group appointments) available in a month for an Acuity appointment type."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+calendar"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "appointment_type_id", Type: core.ConnectionTypeInteger, Label: "Appointment Type ID", Required: true},
	{Name: "month", Type: core.ConnectionTypeString, Label: "Month", Placeholder: "2026-07 (YYYY-MM)", Required: true},
	{Name: "include_unavailable", Type: core.ConnectionTypeBoolean, Label: "Include Full/Unavailable", Placeholder: "Also return classes with no open spots"},
	{Name: "timezone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "America/New_York (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Classes"},
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
	month, err := acuity.RequiredString("month", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	q.Set("appointmentTypeID", fmt.Sprintf("%d", typeID))
	q.Set("month", month)
	acuity.AddFilter(q, inputs, "timezone", "timezone")
	if acuity.OptionalBool("include_unavailable", inputs) {
		q.Set("includeUnavailable", "true")
	}

	items, err := acuity.GetList(userID, apiKey, "/availability/classes", q)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ListResult(items, fmt.Sprintf("Found %d classes in %s", len(items), month)), nil
}
