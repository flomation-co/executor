package monday_user_get_all

import (
	"fmt"

	core "flomation.app/automate/executor"
	monday "flomation.app/automate/executor/actions/monday"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Get Users"
	Description  = "List the users in your Monday.com account."
	Website      = "https://www.flomation.co"
	Icon         = "monday+user"
	Date         = "07/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_token", Type: core.ConnectionTypeSecret, Label: "API Token", Placeholder: "Your Monday.com API token", Required: true},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit", Placeholder: "Maximum number of users to return (1-100)"},
}

var Outputs = [...]core.Connection{
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Users"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "total", Type: core.ConnectionTypeInteger, Label: "Total"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := monday.GetAuth(inputs)
	if err != nil {
		return nil, err
	}
	limitVal, set := monday.OptionalInt("limit", inputs)
	limit := monday.ClampLimit(limitVal, set)
	data, err := monday.GraphQL(auth, `query ($limit: Int) { users (limit: $limit) { id name email enabled } }`,
		map[string]interface{}{"limit": limit})
	if err != nil {
		return monday.ErrorResult(err.Error()), nil
	}
	users := monday.ArrayField(data, "users")
	return monday.ListResult(users, fmt.Sprintf("Retrieved %d user(s)", len(users))), nil
}
