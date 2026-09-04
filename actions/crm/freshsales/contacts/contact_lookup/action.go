// Package contact_lookup implements the Freshsales "Contact: Look Up" action.
package contact_lookup

import (
	"fmt"
	"net/http"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Contact: Look Up"
	Description  = "Find a contact by a unique field such as email, mobile or external ID."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+magnifying-glass"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "q", Type: core.ConnectionTypeString, Label: "Value", Placeholder: "ada@example.com", Required: true},
	{Name: "f", Type: core.ConnectionTypeString, Label: "Field", Placeholder: "email, mobile_number, work_number or external_id", Required: true},
	{Name: "entities", Type: core.ConnectionTypeString, Label: "Entities", Placeholder: "contact"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Record"},
	{Name: "results", Type: core.ConnectionTypeObject, Label: "Matches"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	client, err := freshsales_common.Client(inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	query := freshsales_common.Query(inputs, map[string]string{"q": freshsales_common.OptionalString("q", inputs), "f": freshsales_common.OptionalString("f", inputs), "entities": freshsales_common.OptionalString("entities", inputs)})

	resp, err := client.Do(flow, http.MethodGet, "/lookup", nil, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	contacts := freshsales_common.Obj(resp, "contacts")
	items := freshsales_common.Arr(contacts, "contacts")
	if len(items) == 0 {
		return freshsales_common.ListResult(nil, "No match found"), nil
	}
	first, _ := items[0].(map[string]interface{})
	out := freshsales_common.ObjectResult(first, fmt.Sprintf("Found %s", freshsales_common.NameOf(first)))
	out["results"] = items
	out["count"] = int64(len(items))
	return out, nil
}
