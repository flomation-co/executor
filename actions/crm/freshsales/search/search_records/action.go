// Package search_records implements the Freshsales "Search Records" action.
package search_records

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Search Records"
	Description  = "Search across Freshsales contacts, accounts, deals and users by keyword."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+magnifying-glass"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "q", Type: core.ConnectionTypeString, Label: "Search Term", Placeholder: "Ada", Required: true},
	{Name: "entities", Type: core.ConnectionTypeString, Label: "Entities", Placeholder: "contact,sales_account,deal,user"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Records"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	client, err := freshsales_common.Client(inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	query := freshsales_common.Query(inputs, map[string]string{"q": freshsales_common.OptionalString("q", inputs), "entities": freshsales_common.OptionalString("entities", inputs)})

	resp, err := client.Do(flow, http.MethodGet, "/search", nil, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	items := freshsales_common.Arr(resp, "results")
	if items == nil {
		if raw, ok := resp["response"]; ok {
			_ = raw
		}
	}
	return freshsales_common.ListResult(items, fmt.Sprintf("Search results: %d record(s)", len(items))), nil
}
