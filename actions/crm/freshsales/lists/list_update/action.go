// Package list_update implements the Freshsales "List: Update" action.
package list_update

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
	Name         = "List: Update"
	Description  = "Rename an existing Freshsales marketing list."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+pencil"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "List ID", Placeholder: "12345", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "List Name", Placeholder: "Q4 Prospects", Required: true},
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
	record := map[string]interface{}{}
	freshsales_common.SetString(record, "name", "name", inputs)
	payload := map[string]interface{}{"list": record}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPut, "/lists/"+idValue, payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "list")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Updated list %s", freshsales_common.NameOf(recordOut))), nil
}
