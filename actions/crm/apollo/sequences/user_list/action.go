package user_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Users: List"
	Description  = "List the users (team members) in your Apollo account — handy for owner IDs."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+list"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "25 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Users"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	q := url.Values{}
	if v := apollo_common.OptionalInt("page", inputs); v != nil {
		q.Set("page", fmt.Sprintf("%d", *v))
	}
	if v := apollo_common.OptionalInt("per_page", inputs); v != nil {
		q.Set("per_page", fmt.Sprintf("%d", *v))
	}

	resp, err := apollo_common.NewClient(apiKey).Get(flow, "/users/search", q)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	users := apollo_common.Arr(resp, "users")
	return apollo_common.ListResult(users, fmt.Sprintf("Found %d users", len(users))), nil
}
