// Package note_create implements the Freshsales "Note: Create" action.
package note_create

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
	Name         = "Note: Create"
	Description  = "Attach a note to a Freshsales contact, account or deal."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+plus"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Note", Placeholder: "Spoke to Ada about the rollout", Required: true},
	{Name: "targetable_id", Type: core.ConnectionTypeString, Label: "Record ID", Placeholder: "12345", Required: true},
	{Name: "targetable_type", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Contact, SalesAccount or Deal", Required: true, Options: []core.ConnectionOption{{Name: "Contact", Value: "Contact"}, {Name: "Sales Account", Value: "SalesAccount"}, {Name: "Deal", Value: "Deal"}}},
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

	record := map[string]interface{}{}
	freshsales_common.SetString(record, "description", "description", inputs)
	freshsales_common.SetString(record, "targetable_id", "targetable_id", inputs)
	freshsales_common.SetString(record, "targetable_type", "targetable_type", inputs)
	payload := map[string]interface{}{"note": record}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPost, "/notes", payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "note")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Created note %s", freshsales_common.NameOf(recordOut))), nil
}
