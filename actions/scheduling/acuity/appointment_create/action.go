package scheduling_acuity_appointment_create

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	acuity "flomation.app/automate/executor/actions/scheduling/acuity"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Acuity: Book Appointment"
	Description  = "Book a new Acuity appointment for a client at a given date and time."
	Website      = "https://www.flomation.co"
	Icon         = "acuity+plus"
	Date         = "04/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "Acuity User ID", Required: true},
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Acuity API Key", Placeholder: "Acuity API key (Basic-auth password)", Required: true},
	{Name: "appointment_type_id", Type: core.ConnectionTypeInteger, Label: "Appointment Type ID", Required: true},
	{Name: "datetime", Type: core.ConnectionTypeString, Label: "Date & Time", Placeholder: "2026-07-10T09:00:00-0700 (ISO 8601)", Required: true},
	{Name: "first_name", Type: core.ConnectionTypeString, Label: "First Name", Required: true},
	{Name: "last_name", Type: core.ConnectionTypeString, Label: "Last Name", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "client@example.com", Required: true},
	{Name: "calendar_id", Type: core.ConnectionTypeInteger, Label: "Calendar ID", Placeholder: "Assign to a calendar (optional)"},
	{Name: "phone", Type: core.ConnectionTypeString, Label: "Phone", Placeholder: "Client phone (optional)"},
	{Name: "timezone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "America/New_York (optional)"},
	{Name: "certificate", Type: core.ConnectionTypeString, Label: "Certificate", Placeholder: "Package/coupon code (optional)"},
	{Name: "notes", Type: core.ConnectionTypeText, Label: "Notes", Placeholder: "Appointment notes (optional)"},
	{Name: "fields_json", Type: core.ConnectionTypeObject, Label: "Intake Form Fields (JSON array, advanced)", Placeholder: `[{"id":1,"value":"answer"}]`},
	{Name: "admin", Type: core.ConnectionTypeBoolean, Label: "Admin Booking", Placeholder: "Bypass availability/booking limits"},
	{Name: "no_email", Type: core.ConnectionTypeBoolean, Label: "Suppress Emails", Placeholder: "Do not send confirmation emails"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Appointment ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Appointment"},
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
	firstName, err := acuity.RequiredString("first_name", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	lastName, err := acuity.RequiredString("last_name", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	email, err := acuity.RequiredString("email", inputs)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		"appointmentTypeID": typeID,
		"datetime":          datetime,
		"firstName":         firstName,
		"lastName":          lastName,
		"email":             email,
	}
	if calID, ok := acuity.OptionalInt("calendar_id", inputs); ok {
		body["calendarID"] = calID
	}
	acuity.SetIfString(body, inputs, "phone", "phone")
	acuity.SetIfString(body, inputs, "timezone", "timezone")
	acuity.SetIfString(body, inputs, "certificate", "certificate")
	acuity.SetIfString(body, inputs, "notes", "notes")
	if fields, err := acuity.ParseJSONArray("fields_json", inputs); err != nil {
		return acuity.ErrorResult(err.Error()), nil
	} else if fields != nil {
		body["fields"] = fields
	}

	q := url.Values{}
	if acuity.OptionalBool("admin", inputs) {
		q.Set("admin", "true")
	}
	if acuity.OptionalBool("no_email", inputs) {
		q.Set("noEmail", "true")
	}

	resp, err := acuity.PostObject(userID, apiKey, "/appointments", q, body)
	if err != nil {
		return acuity.ErrorResult(err.Error()), nil
	}
	return acuity.ResourceResult(resp, fmt.Sprintf("Booked appointment for %s at %s", email, datetime)), nil
}
