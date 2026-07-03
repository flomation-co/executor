package scheduling_calendly_event_get_all

import (
	"fmt"
	"net/url"
	"strconv"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Get Many Events"
	Description  = "List scheduled Calendly events (bookings) for you or your organisation, with status, time and invitee filters."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+list"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "scope", Type: core.ConnectionTypeString, Label: "Scope", Options: []core.ConnectionOption{
		{Name: "User", Value: "user"},
		{Name: "Organization", Value: "organization"},
	}},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Active", Value: "active"},
		{Name: "Canceled", Value: "canceled"},
	}},
	{Name: "invitee_email", Type: core.ConnectionTypeString, Label: "Invitee Email", Placeholder: "Only events booked by this invitee (optional)"},
	{Name: "min_start_time", Type: core.ConnectionTypeString, Label: "Start After", Placeholder: "2026-07-01T00:00:00Z (optional)"},
	{Name: "max_start_time", Type: core.ConnectionTypeString, Label: "Start Before", Placeholder: "2026-08-01T00:00:00Z (optional)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Results per page (1-100, default 50)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination and return every event"},
	{Name: "page_token", Type: core.ConnectionTypeString, Label: "Page Token", Placeholder: "Resume from a previous run's next_page_token"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Events"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "next_page_token", Type: core.ConnectionTypeString, Label: "Next Page Token"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := calendly.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	q := url.Values{}
	if err := calendly.ScopeFilter(token, inputs, q); err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	calendly.AddFilter(q, inputs, "status", "status")
	calendly.AddFilter(q, inputs, "invitee_email", "invitee_email")
	calendly.AddFilter(q, inputs, "min_start_time", "min_start_time")
	calendly.AddFilter(q, inputs, "max_start_time", "max_start_time")
	limit, set := calendly.OptionalInt("limit", inputs)
	q.Set("count", strconv.Itoa(calendly.ClampLimit(limit, set)))
	calendly.AddFilter(q, inputs, "page_token", "page_token")

	returnAll := calendly.OptionalBool("return_all", inputs)
	items, next, lastRaw, _, err := calendly.ListResources(token, "/scheduled_events", q, returnAll)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	return calendly.ListResult(items, next, lastRaw, fmt.Sprintf("Retrieved %d scheduled events", len(items))), nil
}
