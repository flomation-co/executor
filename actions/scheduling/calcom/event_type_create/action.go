package scheduling_calcom_event_type_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Create Event Type"
	Description  = "Create a new Cal.com event type (a bookable meeting slot definition)."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+plus"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "e.g. Discovery Call", Required: true},
	{Name: "slug", Type: core.ConnectionTypeString, Label: "Slug", Placeholder: "e.g. discovery-call", Required: true},
	{Name: "length_minutes", Type: core.ConnectionTypeInteger, Label: "Length (minutes)", Placeholder: "e.g. 30", Required: true},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "Shown on the booking page (optional)"},
	{Name: "disable_guests", Type: core.ConnectionTypeBoolean, Label: "Disable Guests"},
	{Name: "minimum_booking_notice", Type: core.ConnectionTypeInteger, Label: "Minimum Booking Notice (minutes)"},
	{Name: "schedule_id", Type: core.ConnectionTypeInteger, Label: "Availability Schedule ID", Placeholder: "Attach an availability schedule (optional)"},
	{Name: "extra_json", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON, advanced)", Placeholder: `{"locations":[{"type":"integration","integration":"cal-video"}]}`},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Event Type ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Event Type"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calcom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	title, err := calcom.RequiredString("title", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	slug, err := calcom.RequiredString("slug", inputs)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	length, ok := calcom.OptionalInt("length_minutes", inputs)
	if !ok || length <= 0 {
		return calcom.ErrorResult("length_minutes is required"), nil
	}

	body := map[string]interface{}{
		"title":           title,
		"slug":            slug,
		"lengthInMinutes": length,
	}
	calcom.SetIfString(body, inputs, "description", "description")
	calcom.SetIfBoolPresent(body, inputs, "disableGuests", "disable_guests")
	calcom.SetIfInt(body, inputs, "minimumBookingNotice", "minimum_booking_notice")
	calcom.SetIfInt(body, inputs, "scheduleId", "schedule_id")
	if extra, err := calcom.ParseJSONObject("extra_json", inputs); err != nil {
		return calcom.ErrorResult(err.Error()), nil
	} else {
		for k, v := range extra {
			body[k] = v
		}
	}

	resp, err := calcom.PostResource(token, "/event-types", calcom.VersionEventTypes, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Created event type %q", title)), nil
}
