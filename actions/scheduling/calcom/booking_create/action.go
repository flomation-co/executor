package scheduling_calcom_booking_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Create Booking"
	Description  = "Book a Cal.com event type for an attendee at a given start time."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+plus"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "event_type_id", Type: core.ConnectionTypeInteger, Label: "Event Type ID", Placeholder: "The event type to book", Required: true},
	{Name: "start", Type: core.ConnectionTypeString, Label: "Start Time", Placeholder: "2026-07-10T09:00:00Z (ISO 8601, UTC)", Required: true},
	{Name: "attendee_name", Type: core.ConnectionTypeString, Label: "Attendee Name", Required: true},
	{Name: "attendee_email", Type: core.ConnectionTypeString, Label: "Attendee Email", Placeholder: "person@example.com", Required: true},
	{Name: "attendee_timezone", Type: core.ConnectionTypeString, Label: "Attendee Time Zone", Placeholder: "Europe/London", Required: true},
	{Name: "attendee_phone", Type: core.ConnectionTypeString, Label: "Attendee Phone", Placeholder: "+447700900000 (optional)"},
	{Name: "guests", Type: core.ConnectionTypeString, Label: "Guest Emails", Placeholder: "Comma-separated additional guest emails (optional)"},
	{Name: "length_minutes", Type: core.ConnectionTypeInteger, Label: "Length (minutes)", Placeholder: "Override the event type length (optional)"},
	{Name: "metadata_json", Type: core.ConnectionTypeObject, Label: "Metadata (JSON, advanced)", Placeholder: `{"source":"flomation"}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Booking UID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Booking"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	eventTypeID, err := calcom.RequiredInt("event_type_id", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	start, err := calcom.RequiredString("start", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	name, err := calcom.RequiredString("attendee_name", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	email, err := calcom.RequiredString("attendee_email", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	tz, err := calcom.RequiredString("attendee_timezone", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}

	attendee := map[string]interface{}{"name": name, "email": email, "timeZone": tz}
	if phone := calcom.OptionalString("attendee_phone", inputs); phone != "" {
		attendee["phoneNumber"] = phone
	}

	body := map[string]interface{}{
		"start":       start,
		"eventTypeId": eventTypeID,
		"attendee":    attendee,
	}
	if guests := calcom.OptionalStringSlice("guests", inputs); len(guests) > 0 {
		body["guests"] = guests
	}
	calcom.SetIfInt(body, inputs, "lengthInMinutes", "length_minutes")
	if meta, err := calcom.ParseJSONObject("metadata_json", inputs); err != nil {
		return calcom.ErrorResult(err.Error()), nil
	} else if meta != nil {
		body["metadata"] = meta
	}

	resp, err := calcom.PostResource(token, "/bookings", calcom.VersionBookings, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Booked %s for %s", start, email)), nil
}
