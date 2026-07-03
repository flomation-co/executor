package scheduling_calcom_event_type_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Update Event Type"
	Description  = "Update fields of an existing Cal.com event type. Only supplied fields change."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+pencil"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "event_type_id", Type: core.ConnectionTypeInteger, Label: "Event Type ID", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "New title (optional)"},
	{Name: "slug", Type: core.ConnectionTypeString, Label: "Slug", Placeholder: "New slug (optional)"},
	{Name: "length_minutes", Type: core.ConnectionTypeInteger, Label: "Length (minutes)", Placeholder: "New length (optional)"},
	{Name: "description", Type: core.ConnectionTypeText, Label: "Description", Placeholder: "New description (optional)"},
	{Name: "disable_guests", Type: core.ConnectionTypeBoolean, Label: "Disable Guests"},
	{Name: "minimum_booking_notice", Type: core.ConnectionTypeInteger, Label: "Minimum Booking Notice (minutes)"},
	{Name: "schedule_id", Type: core.ConnectionTypeInteger, Label: "Availability Schedule ID"},
	{Name: "extra_json", Type: core.ConnectionTypeObject, Label: "Additional Fields (JSON, advanced)", Placeholder: `{"color":{"lightThemeHex":"#292929"}}`},
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
	id, ok := calcom.OptionalInt("event_type_id", inputs)
	if !ok {
		return calcom.ErrorResult("event_type_id is required"), nil
	}

	body := map[string]interface{}{}
	calcom.SetIfString(body, inputs, "title", "title")
	calcom.SetIfString(body, inputs, "slug", "slug")
	calcom.SetIfInt(body, inputs, "lengthInMinutes", "length_minutes")
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
	if len(body) == 0 {
		return calcom.ErrorResult("no fields to update: supply at least one field"), nil
	}

	resp, err := calcom.PatchResource(token, fmt.Sprintf("/event-types/%d", id), calcom.VersionEventTypes, body)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ResourceResult(resp, fmt.Sprintf("Updated event type %d", id)), nil
}
