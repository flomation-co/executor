// Package list_get_contacts implements the Freshsales "List: Get Contacts" action.
package list_get_contacts

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List: Get Contacts"
	Description  = "Fetch the contacts belonging to a Freshsales marketing list."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+people-group"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "List ID", Placeholder: "12345", Required: true},
	{Name: "page", Type: core.ConnectionTypeInteger, Label: "Page", Placeholder: "1"},
	{Name: "per_page", Type: core.ConnectionTypeInteger, Label: "Per Page", Placeholder: "25"},
	{Name: "sort", Type: core.ConnectionTypeString, Label: "Sort By", Placeholder: "updated_at"},
	{Name: "sort_type", Type: core.ConnectionTypeString, Label: "Sort Direction", Placeholder: "asc or desc", Options: []core.ConnectionOption{{Name: "Ascending", Value: "asc"}, {Name: "Descending", Value: "desc"}}},
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

	idValue, err := freshsales_common.RequiredString("id", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	query := freshsales_common.Query(inputs, map[string]string{})

	resp, err := client.Do(flow, http.MethodGet, "/lists/"+idValue+"/contacts", nil, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	items := freshsales_common.Arr(resp, "contacts")
	return freshsales_common.ListResult(items, fmt.Sprintf("List contacts: %d record(s)", len(items))), nil
}
