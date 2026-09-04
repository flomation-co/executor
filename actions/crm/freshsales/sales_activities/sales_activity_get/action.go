// Package sales_activity_get implements the Freshsales "Sales Activity: Get" action.
package sales_activity_get

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
	Name         = "Sales Activity: Get"
	Description  = "Fetch a single Freshsales sales activity by ID."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+magnifying-glass"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Sales Activity ID", Placeholder: "12345", Required: true},
	{Name: "include", Type: core.ConnectionTypeString, Label: "Include"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Record"},
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
	var query url.Values

	resp, err := client.Do(flow, http.MethodGet, "/sales_activities/"+idValue, nil, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "sales_activity")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Fetched sales activity %s", freshsales_common.NameOf(recordOut))), nil
}
