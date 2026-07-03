package scheduling_calcom_booking_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Get Many Bookings"
	Description  = "List Cal.com bookings, filtered by status, attendee, event type or date range."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+list"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Upcoming", Value: "upcoming"},
		{Name: "Recurring", Value: "recurring"},
		{Name: "Past", Value: "past"},
		{Name: "Cancelled", Value: "cancelled"},
		{Name: "Unconfirmed", Value: "unconfirmed"},
	}},
	{Name: "attendee_email", Type: core.ConnectionTypeString, Label: "Attendee Email", Placeholder: "Only bookings for this attendee (optional)"},
	{Name: "attendee_name", Type: core.ConnectionTypeString, Label: "Attendee Name", Placeholder: "Only bookings for this attendee name (optional)"},
	{Name: "event_type_id", Type: core.ConnectionTypeInteger, Label: "Event Type ID", Placeholder: "Only bookings for this event type (optional)"},
	{Name: "after_start", Type: core.ConnectionTypeString, Label: "Starts After", Placeholder: "2026-07-01T00:00:00Z (optional)"},
	{Name: "before_end", Type: core.ConnectionTypeString, Label: "Ends Before", Placeholder: "2026-08-01T00:00:00Z (optional)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Results per page (1-250, default 100)"},
	{Name: "skip", Type: core.ConnectionTypeInteger, Label: "Skip", Placeholder: "Offset to resume from a previous run's next_skip"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination and return every booking"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Bookings"},
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

	q := url.Values{}
	calcom.AddFilter(q, inputs, "status", "status")
	calcom.AddFilter(q, inputs, "attendeeEmail", "attendee_email")
	calcom.AddFilter(q, inputs, "attendeeName", "attendee_name")
	if id, ok := calcom.OptionalInt("event_type_id", inputs); ok {
		q.Set("eventTypeId", fmt.Sprintf("%d", id))
	}
	calcom.AddFilter(q, inputs, "afterStart", "after_start")
	calcom.AddFilter(q, inputs, "beforeEnd", "before_end")

	limit, set := calcom.OptionalInt("limit", inputs)
	skip, _ := calcom.OptionalInt("skip", inputs)
	returnAll := calcom.OptionalBool("return_all", inputs)

	items, next, _, err := calcom.ListResources(token, "/bookings", calcom.VersionBookings, q, calcom.ClampLimit(limit, set), skip, returnAll)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ListResult(items, next, fmt.Sprintf("Retrieved %d bookings", len(items))), nil
}
