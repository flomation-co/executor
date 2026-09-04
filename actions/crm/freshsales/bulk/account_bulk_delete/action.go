// Package account_bulk_delete implements the Freshsales "Bulk: Delete Accounts" action.
package account_bulk_delete

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Bulk: Delete Accounts"
	Description  = "Delete many accounts in one call by ID."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+trash"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "ids", Type: core.ConnectionTypeString, Label: "IDs", Placeholder: "Comma-separated ids", Required: true},
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

	ids := freshsales_common.IDList("ids", inputs)
	if len(ids) == 0 {
		return freshsales_common.ErrorResult("at least one ID is required"), nil
	}
	payload := map[string]interface{}{"selected_ids": ids}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPost, "/sales_accounts/bulk_destroy", payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	return freshsales_common.PlainResult(resp, "Bulk delete accounts"), nil
}
