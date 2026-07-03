package scheduling_calendly_membership_get_all

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
	Name         = "Calendly: Get Many Members"
	Description  = "List the members of your Calendly organisation, optionally filtered by email."
	Website      = "https://www.flomation.co"
	Icon         = "calendly+user-group"
	Date         = "03/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Calendly Connection", Placeholder: "Calendly credential or Personal Access Token", Required: true},
	{Name: "email", Type: core.ConnectionTypeString, Label: "Email", Placeholder: "Only members with this email (optional)"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Results per page (1-100, default 50)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Follow pagination and return every member"},
	{Name: "page_token", Type: core.ConnectionTypeString, Label: "Page Token", Placeholder: "Resume from a previous run's next_page_token"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Memberships"},
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

	_, orgURI, err := calendly.CurrentUser(token)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	q.Set("organization", orgURI)
	calendly.AddFilter(q, inputs, "email", "email")
	limit, set := calendly.OptionalInt("limit", inputs)
	q.Set("count", strconv.Itoa(calendly.ClampLimit(limit, set)))
	calendly.AddFilter(q, inputs, "page_token", "page_token")

	returnAll := calendly.OptionalBool("return_all", inputs)
	items, next, lastRaw, _, err := calendly.ListResources(token, "/organization_memberships", q, returnAll)
	if err != nil {
		return calendly.ErrorResult(err.Error()), nil
	}
	return calendly.ListResult(items, next, lastRaw, fmt.Sprintf("Retrieved %d organisation members", len(items))), nil
}
