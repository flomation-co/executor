package scheduling_calendly_event_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Get Event"
	Description  = "Retrieve a single scheduled Calendly event (booking) by its ID or URI."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+magnifying-glass"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event", Placeholder: "Scheduled event ID or URI", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Event URI"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Event"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calendly.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := calendly.RequiredString("event", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}

	uuid := calendly.ExtractUUID(id)
	resp, err := calendly.GetResource(token, "/scheduled_events/"+url.PathEscape(uuid), nil)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	return calendly.ResourceResult(resp, fmt.Sprintf("Retrieved scheduled event %s", uuid)), nil
}
