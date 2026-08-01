package deal_list

import (
	"fmt"
	"net/url"

	core "flomation.app/automate/executor"
	apollo_common "flomation.app/automate/executor/actions/crm/apollo"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Deals: List"
	Description  = "List the deals (opportunities) in your Apollo CRM, sorted by amount or status."
	Website      = "https://www.flomation.co"
	Icon         = "apollo+list"
	Date         = "01/08/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Apollo API Key", Placeholder: "${secrets.ApolloApiKey}", Required: true},
	{Name: "sort_by_field", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "amount", Options: []core.ConnectionOption{
		{Name: "Amount", Value: "amount"},
		{Name: "Is Closed", Value: "is_closed"},
		{Name: "Is Won", Value: "is_won"},
	}},
	{Name: "sort_ascending", Type: core.ConnectionTypeBoolean, Label: "Sort Ascending"},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "25 (max 100)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Deals"},
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
	if v := apollo_common.OptionalString("sort_by_field", inputs); v != "" {
		q.Set("sort_by_field", v)
	}
	if b := apollo_common.OptionalBool("sort_ascending", inputs); b != nil {
		q.Set("sort_ascending", fmt.Sprintf("%t", *b))
	}
	if v := apollo_common.OptionalInt("page", inputs); v != nil {
		q.Set("page", fmt.Sprintf("%d", *v))
	}
	if v := apollo_common.OptionalInt("per_page", inputs); v != nil {
		q.Set("per_page", fmt.Sprintf("%d", *v))
	}

	resp, err := apollo_common.NewClient(apiKey).Get(flow, "/opportunities/search", q)
	if err != nil {
		return apollo_common.MapError(err), nil
	}

	deals := apollo_common.Arr(resp, "opportunities")
	return apollo_common.ListResult(deals, fmt.Sprintf("Found %d deals", len(deals))), nil
}
