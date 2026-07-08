package helpdesk_intercom_admin_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Get Many Admins"
	Description  = "List the admins (teammates) in your Intercom workspace, including their names, emails, and away status."
	Website      = "https://www.flomation.co"
	Icon         = "intercom+list"
	Date         = "08/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "Access Token", Placeholder: "Your Intercom access token (Developer Hub → Authentication)", Required: true},
	{
		Name:  "region",
		Type:  core.ConnectionTypeString,
		Label: "Region",
		Options: []core.ConnectionOption{
			{Name: "US (default)", Value: "us"},
			{Name: "Europe", Value: "eu"},
			{Name: "Australia", Value: "au"},
		},
	},
	{Name: "limit", Type: core.ConnectionTypeString, Label: "Limit", Placeholder: "Max results (default 50)"},
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Fetch every admin (ignores Limit)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Admins"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := intercom.GetAuth(inputs)
	if err != nil {
		return nil, err
	}

	// GET /admins returns the whole team in one response (no pagination), so
	// Limit is applied client-side and Return All simply skips the trim.
	items, _, err := intercom.ListPage(auth, "/admins", nil, "admins")
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	returnAll, _ := intercom.OptionalBoolSet("return_all", inputs)
	if !returnAll {
		limit, limitSet := intercom.OptionalInt("limit", inputs)
		if max := intercom.ClampLimit(limit, limitSet); len(items) > max {
			items = items[:max]
		}
	}
	return intercom.ListResult(items, fmt.Sprintf("Retrieved %d admin(s)", len(items))), nil
}
