package scheduling_calendly_event_cancel

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Cancel Event"
	Description  = "Cancel a scheduled Calendly event (booking), optionally with a reason shown to the invitee."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+xmark"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Event", Placeholder: "Scheduled event ID or URI", Required: true},
	{Name: "reason", Type: core.ConnectionTypeText, Label: "Reason", Placeholder: "Cancellation reason shown to the invitee (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Cancellation URI"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Cancellation"},
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

	body := map[string]interface{}{}
	if reason := calendly.OptionalString("reason", inputs); reason != "" {
		body["reason"] = reason
	}

	uuid := calendly.ExtractUUID(id)
	resp, err := calendly.PostResource(token, "/scheduled_events/"+url.PathEscape(uuid)+"/cancellation", body)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	return calendly.ResourceResult(resp, fmt.Sprintf("Canceled scheduled event %s", uuid)), nil
}
