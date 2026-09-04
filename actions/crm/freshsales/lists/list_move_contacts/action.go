// Package list_move_contacts implements the Freshsales "List: Move Contacts" action.
package list_move_contacts

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "List: Move Contacts"
	Description  = "Move contacts from one Freshsales marketing list to another."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+arrow-right"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Source List ID", Placeholder: "12345", Required: true},
	{Name: "target_list_id", Type: core.ConnectionTypeString, Label: "Target List ID", Placeholder: "67890", Required: true},
	{Name: "contact_ids", Type: core.ConnectionTypeString, Label: "Contact IDs", Placeholder: "Comma-separated ids", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Response"},
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
	ids := freshsales_common.IDList("contact_ids", inputs)
	if len(ids) == 0 {
		return freshsales_common.ErrorResult("at least one contact ID is required"), nil
	}
	payload := map[string]interface{}{"ids": ids}
	if t := freshsales_common.OptionalString("target_list_id", inputs); t != "" {
		payload["target_list_id"] = t
	}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPost, "/lists/"+idValue+"/move_contacts", payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	return freshsales_common.PlainResult(resp, "Moved contacts"), nil
}
