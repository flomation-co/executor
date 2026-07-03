package scheduling_calendly_scheduling_link_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Create Scheduling Link"
	Description  = "Create a single-use scheduling link for an event type — the link expires after one booking."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+plus"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type", Placeholder: "Event type ID or URI to create the link for", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "booking_url", Type: core.ConnectionTypeString, Label: "Booking URL"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Scheduling Link"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calendly.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	eventType, err := calendly.RequiredString("event_type", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}

	body := map[string]interface{}{
		// Calendly currently only supports single-use links (max_event_count 1).
		"max_event_count": 1,
		"owner":           calendly.ResourceURI(eventType, "event_types"),
		"owner_type":      "EventType",
	}
	resp, err := calendly.PostResource(token, "/scheduling_links", body)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}

	resource, _ := resp["resource"].(map[string]interface{})
	bookingURL := ""
	if resource != nil {
		bookingURL, _ = resource["booking_url"].(string)
	}
	out := calendly.ResourceResult(resp, fmt.Sprintf("Created single-use scheduling link %s", bookingURL))
	out["booking_url"] = bookingURL
	return out, nil
}
