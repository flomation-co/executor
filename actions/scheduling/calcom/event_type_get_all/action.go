package scheduling_calcom_event_type_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calcom "flomation.app/automate/executor/actions/scheduling/calcom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Cal.com: Get Many Event Types"
	Description  = "List Cal.com event types, optionally filtered by username or slug."
	Website      = "https://www.flomation.co"
	Icon         = "calcom+list"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Cal.com Connection", Placeholder: "Cal.com API key (cal_live_...) or connected credential", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Placeholder: "Filter to a user's event types (optional)"},
	{Name: "event_slug", Type: core.ConnectionTypeString, Label: "Event Slug", Placeholder: "Filter to a single event type slug (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Event Types"},
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
	calcom.AddFilter(q, inputs, "username", "username")
	calcom.AddFilter(q, inputs, "eventSlug", "event_slug")

	items, next, _, err := calcom.ListResources(token, "/event-types", calcom.VersionEventTypes, q, 0, 0, false)
	if err != nil {
		return calcom.ErrorResult(err.Error()), nil
	}
	return calcom.ListResult(items, next, fmt.Sprintf("Retrieved %d event types", len(items))), nil
}
