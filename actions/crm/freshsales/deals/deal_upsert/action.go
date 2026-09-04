// Package deal_upsert implements the Freshsales "Deal: Upsert" action.
package deal_upsert

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
	Name         = "Deal: Upsert"
	Description  = "Create or update a deal matched on a unique identifier."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+copy"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "unique_identifier", Type: core.ConnectionTypeString, Label: "Match On (JSON)", Placeholder: `{"email":"ada@example.com"}`},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Deal Name", Placeholder: "Q4 platform rollout", Required: true},
	{Name: "amount", Type: core.ConnectionTypeString, Label: "Deal Value", Placeholder: "25000"},
	{Name: "sales_account_id", Type: core.ConnectionTypeInteger, Label: "Account ID"},
	{Name: "deal_stage_id", Type: core.ConnectionTypeInteger, Label: "Deal Stage ID"},
	{Name: "deal_type_id", Type: core.ConnectionTypeInteger, Label: "Deal Type ID"},
	{Name: "deal_reason_id", Type: core.ConnectionTypeInteger, Label: "Deal Reason ID"},
	{Name: "deal_payment_status_id", Type: core.ConnectionTypeInteger, Label: "Payment Status ID"},
	{Name: "currency_id", Type: core.ConnectionTypeInteger, Label: "Currency ID"},
	{Name: "owner_id", Type: core.ConnectionTypeInteger, Label: "Owner ID"},
	{Name: "campaign_id", Type: core.ConnectionTypeInteger, Label: "Campaign ID"},
	{Name: "lead_source_id", Type: core.ConnectionTypeInteger, Label: "Lead Source ID"},
	{Name: "expected_close", Type: core.ConnectionTypeString, Label: "Expected Close Date", Placeholder: "2026-12-31"},
	{Name: "probability", Type: core.ConnectionTypeInteger, Label: "Probability (%)", Placeholder: "60"},
	{Name: "contacts_added_list", Type: core.ConnectionTypeString, Label: "Contact IDs", Placeholder: "Comma-separated contact ids to associate"},
	{Name: "fields", Type: core.ConnectionTypeText, Label: "Additional Fields (JSON)", Placeholder: `{"custom_field":{"cf_region":"EMEA"}}`},
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
	freshsales_common.SetString(record, "name", "name", inputs)
	freshsales_common.SetNumber(record, "amount", "amount", inputs)
	freshsales_common.SetInt(record, "sales_account_id", "sales_account_id", inputs)
	freshsales_common.SetInt(record, "deal_stage_id", "deal_stage_id", inputs)
	freshsales_common.SetInt(record, "deal_type_id", "deal_type_id", inputs)
	freshsales_common.SetInt(record, "deal_reason_id", "deal_reason_id", inputs)
	freshsales_common.SetInt(record, "deal_payment_status_id", "deal_payment_status_id", inputs)
	freshsales_common.SetInt(record, "currency_id", "currency_id", inputs)
	freshsales_common.SetInt(record, "owner_id", "owner_id", inputs)
	freshsales_common.SetInt(record, "campaign_id", "campaign_id", inputs)
	freshsales_common.SetInt(record, "lead_source_id", "lead_source_id", inputs)
	freshsales_common.SetString(record, "expected_close", "expected_close", inputs)
	freshsales_common.SetInt(record, "probability", "probability", inputs)
	freshsales_common.SetIDList(record, "contacts_added_list", "contacts_added_list", inputs)
	extra, err := freshsales_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	freshsales_common.MergeFields(record, extra)
	payload := map[string]interface{}{"deal": record}
	unique, err := freshsales_common.ParseJSONObject("unique_identifier", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	if unique != nil {
		payload["unique_identifier"] = unique
	}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPost, "/deals/upsert", payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "deal")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Upserted deal %s", freshsales_common.NameOf(recordOut))), nil
}
