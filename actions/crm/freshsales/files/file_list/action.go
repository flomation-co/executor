// Package file_list implements the Freshsales "Files: List For Record" action.
package file_list

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
	Name         = "Files: List For Record"
	Description  = "List the files and links attached to a Freshsales record."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+list"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "targetable_id", Type: core.ConnectionTypeString, Label: "Record ID", Placeholder: "12345", Required: true},
	{Name: "targetable_type", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Contact, SalesAccount or Deal", Required: true, Options: []core.ConnectionOption{{Name: "Contact", Value: "Contact"}, {Name: "Sales Account", Value: "SalesAccount"}, {Name: "Deal", Value: "Deal"}}},
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

	targetable_typeValue, err := freshsales_common.RequiredString("targetable_type", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	targetable_idValue, err := freshsales_common.RequiredString("targetable_id", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	var query url.Values

	segment := freshsales_common.TargetablePath(targetable_typeValue)
	if segment == "" {
		return freshsales_common.ErrorResult("record type must be Contact, SalesAccount or Deal"), nil
	}
	resp, err := client.Do(flow, http.MethodGet, "/"+segment+"/"+targetable_idValue+"/documents", nil, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	items := freshsales_common.Arr(resp, "documents")
	return freshsales_common.ListResult(items, fmt.Sprintf("Attached documents: %d record(s)", len(items))), nil
}
