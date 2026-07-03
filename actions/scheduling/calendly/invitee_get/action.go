package scheduling_calendly_invitee_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Get Invitee"
	Description  = "Retrieve a single invitee of a scheduled Calendly event, including their answers to booking questions."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+magnifying-glass"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event", Placeholder: "Scheduled event ID or URI", Required: true},
	{Name: "invitee", Type: core.ConnectionTypeString, Label: "Invitee", Placeholder: "Invitee ID or URI", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Invitee URI"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Invitee"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calendly.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	eventID, err := calendly.RequiredString("event", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	inviteeID, err := calendly.RequiredString("invitee", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}

	eventUUID := calendly.ExtractUUID(eventID)
	inviteeUUID := calendly.ExtractUUID(inviteeID)
	resp, err := calendly.GetResource(token, "/scheduled_events/"+url.PathEscape(eventUUID)+"/invitees/"+url.PathEscape(inviteeUUID), nil)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	return calendly.ResourceResult(resp, fmt.Sprintf("Retrieved invitee %s", inviteeUUID)), nil
}
