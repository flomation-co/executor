package scheduling_calendly_busy_times_get_all

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	calendly "flomation.app/automate/executor/actions/scheduling/calendly"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Calendly: Get Busy Times"
	Description  = "List a Calendly user's busy periods (internal and external calendar events) within a date range of up to 7 days."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+clock"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time", Placeholder: "2026-07-03T00:00:00Z", Required: true},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time", Placeholder: "2026-07-10T00:00:00Z (max 7 days after start)", Required: true},
	{Name: "user", Type: core.ConnectionTypeString, Label: "User URI", Placeholder: "Defaults to the connected user (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Busy Times"},
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
	start, err := calendly.RequiredString("start_time", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	end, err := calendly.RequiredString("end_time", inputs)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
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
	q.Set("start_time", start)
	q.Set("end_time", end)
	items, next, lastRaw, _, err := calendly.ListResources(token, "/user_busy_times", q, false)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	out := calendly.ListResult(items, next, lastRaw, fmt.Sprintf("Retrieved %d busy periods", len(items)))
	delete(out, "next_page_token")
	return out, nil
}
