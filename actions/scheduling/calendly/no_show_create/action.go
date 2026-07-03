package scheduling_calendly_no_show_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Mark Invitee No-Show"
	Description  = "Mark an invitee of a scheduled Calendly event as a no-show."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+ban"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "invitee", Type: core.ConnectionTypeString, Label: "Invitee URI", Placeholder: "https://api.calendly.com/scheduled_events/.../invitees/...", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "No-Show URI"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "No-Show"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calendly.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	invitee, err := calendly.RequiredString("invitee", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}

	// Marking a no-show requires the full invitee URI (it is nested under its
	// event, so a bare UUID is ambiguous).
	body := map[string]interface{}{"invitee": invitee}
	resp, err := calendly.PostResource(token, "/invitee_no_shows", body)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	return calendly.ResourceResult(resp, fmt.Sprintf("Marked invitee as no-show")), nil
}
