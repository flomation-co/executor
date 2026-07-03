package scheduling_calendly_event_type_get

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Get Event Type"
	Description  = "Retrieve a single Calendly event type (meeting template) by its ID or URI."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+magnifying-glass"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "event_type", Type: core.ConnectionTypeString, Label: "Event Type", Placeholder: "Event type ID or URI", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Event Type URI"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Event Type"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calendly.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	id, err := calendly.RequiredString("event_type", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}

	uuid := calendly.ExtractUUID(id)
	resp, err := calendly.GetResource(token, "/event_types/"+url.PathEscape(uuid), nil)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	return calendly.ResourceResult(resp, fmt.Sprintf("Retrieved event type %s", uuid)), nil
}
