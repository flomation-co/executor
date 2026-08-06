package account_search

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Accounts: Search"
	Description  = "Search the accounts saved in your Apollo CRM by name, stage and list."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+magnifying-glass"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "q_organization_name", Type: core.ConnectionTypeString, Label: "Account Name", Placeholder: "Search by account name"},
	{Name: "account_stage_ids", Type: core.ConnectionTypeString, Label: "Stage IDs", Placeholder: "Comma-separated account stage IDs"},
	{Name: "account_label_ids", Type: core.ConnectionTypeString, Label: "List IDs", Placeholder: "Comma-separated list (label) IDs"},
	{Name: "sort_by_field", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "e.g. account_last_activity_date"},
	{Name: "sort_ascending", Type: core.ConnectionTypeBoolean, Label: "Sort Ascending"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "25 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Accounts"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	apiKey, err := apollo_common.GetAPIKey(inputs)
	if err != nil {
		return apollo_common.ErrorResult(err.Error()), nil
	}

	// Apollo reads account-search filters from the URL query string, not the body.
	q := url.Values{}
	apollo_common.AddQueryString(q, "q_organization_name", "q_organization_name", inputs)
	apollo_common.AddQueryList(q, "account_stage_ids", "account_stage_ids", inputs)
	apollo_common.AddQueryList(q, "account_label_ids", "account_label_ids", inputs)
	apollo_common.AddQueryString(q, "sort_by_field", "sort_by_field", inputs)
	apollo_common.AddQueryBool(q, "sort_ascending", "sort_ascending", inputs)
	apollo_common.AddQueryInt(q, "page", "page", inputs)
	apollo_common.AddQueryInt(q, "per_page", "per_page", inputs)

	resp, err := apollo_common.NewClient(apiKey).PostQuery(flow, "/accounts/search", q)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	accounts := apollo_common.Arr(resp, "accounts")
	return apollo_common.ListResult(accounts, fmt.Sprintf("Found %d accounts", len(accounts))), nil
}
