// Package contact_list_filters implements the Freshsales "Contact: List Views" action.
package contact_list_filters

import (
	"fmt"
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Contact: List Views"
	Description  = "List the saved views (filters) available for contacts. Use a view ID with List By View."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+list"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
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

	var query url.Values

	resp, err := client.Do(flow, http.MethodGet, "/contacts/filters", nil, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	items := freshsales_common.Arr(resp, "filters")
	return freshsales_common.ListResult(items, fmt.Sprintf("Contact views: %d record(s)", len(items))), nil
}
