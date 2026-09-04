// Package deal_list_by_view implements the Freshsales "Deal: List By View" action.
package deal_list_by_view

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Deal: List By View"
	Description  = "List deals in a saved view. Get the view ID from Deal: List Views."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+list"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "view_id", Type: core.ConnectionTypeString, Label: "View ID", Placeholder: "Get this from the List Views action", Required: true},
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

	view_idValue, err := freshsales_common.RequiredString("view_id", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	query := freshsales_common.Query(inputs, map[string]string{})

	resp, err := client.Do(flow, http.MethodGet, "/deals/view/"+view_idValue, nil, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	items := freshsales_common.Arr(resp, "deals")
	return freshsales_common.ListResult(items, fmt.Sprintf("Deal list: %d record(s)", len(items))), nil
}
