package scheduling_calendly_availability_schedule_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Get Availability Schedules"
	Description  = "List a Calendly user's availability schedules (working hours). Defaults to the connected user."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+clock"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "user", Type: core.ConnectionTypeString, Label: "User URI", Placeholder: "Defaults to the connected user (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Availability Schedules"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
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

	user := calendly.OptionalString("user", inputs)
	if user == "" {
		userURI, _, err := calendly.CurrentUser(token)
		if err != nil {
			return calendly.ErrorResult(err.Error()), nil
		}
		user = userURI
	} else {
		user = calendly.ResourceURI(user, "users")
	}

	q := url.Values{}
	q.Set("user", user)
	// /user_availability_schedules is unpaginated — a user has a handful of
	// schedules at most — so a single ListResources page drains it.
	items, next, lastRaw, _, err := calendly.ListResources(token, "/user_availability_schedules", q, false)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	out := calendly.ListResult(items, next, lastRaw, fmt.Sprintf("Retrieved %d availability schedules", len(items)))
	delete(out, "next_page_token")
	return out, nil
}
