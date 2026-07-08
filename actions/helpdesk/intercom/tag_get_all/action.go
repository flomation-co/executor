package helpdesk_intercom_tag_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	intercom "flomation.app/automate/executor/actions/helpdesk/intercom"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Intercom: Get Many Tags"
	Description  = "List the tags in your Intercom workspace — the labels you apply to contacts, companies, and conversations."
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
	{Name: "return_all", Type: core.ConnectionTypeBoolean, Label: "Return All", Placeholder: "Return every tag (ignores Limit)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Tags"},
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

	// GET /tags is unpaginated — every tag in the workspace comes back in one
	// response — so Limit is applied here rather than as a per_page param.
	items, _, err := intercom.ListPage(auth, "/tags", nil, "data")
	if err != nil {
		return intercom.ErrorResult(err.Error()), nil
	}
	returnAll, _ := intercom.OptionalBoolSet("return_all", inputs)
	limit, limitSet := intercom.OptionalInt("limit", inputs)
	if maxItems := intercom.ClampLimit(limit, limitSet); !returnAll && len(items) > maxItems {
		items = items[:maxItems]
	}
	return intercom.ListResult(items, fmt.Sprintf("Retrieved %d tag(s)", len(items))), nil
}
