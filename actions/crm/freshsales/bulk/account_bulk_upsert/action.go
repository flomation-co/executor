// Package account_bulk_upsert implements the Freshsales "Bulk: Upsert Accounts" action.
package account_bulk_upsert

import (
	"net/http"
	"net/url"

	core "flomation.app/automate/executor"
	freshsales_common "flomation.app/automate/executor/actions/crm/freshsales"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Bulk: Upsert Accounts"
	Description  = "Create or update many accounts in one call. Cheaper against the hourly rate limit."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+copy"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "records", Type: core.ConnectionTypeString, Label: "Records (JSON array)", Placeholder: `[{"first_name":"Ada","email":"ada@example.com"}]`, Required: true},
	{Name: "unique_identifier", Type: core.ConnectionTypeString, Label: "Match On (JSON)", Placeholder: `{"email":"email"}`},
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

	records, err := freshsales_common.ParseJSONArray("records", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	if len(records) == 0 {
		return freshsales_common.ErrorResult("records must be a non-empty JSON array"), nil
	}
	payload := map[string]interface{}{"sales_accounts": records}
	unique, err := freshsales_common.ParseJSONObject("unique_identifier", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	if unique != nil {
		payload["unique_identifier"] = unique
	}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPost, "/sales_accounts/bulk_upsert", payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	return freshsales_common.PlainResult(resp, "Bulk upsert accounts"), nil
}
