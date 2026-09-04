// Package sales_activity_create implements the Freshsales "Sales Activity: Create" action.
package sales_activity_create

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
	Name         = "Sales Activity: Create"
	Description  = "Create a Freshsales sales activity."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+plus"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "Intro call", Required: true},
	{Name: "sales_activity_type_id", Type: core.ConnectionTypeInteger, Label: "Activity Type ID"},
	{Name: "sales_activity_outcome_id", Type: core.ConnectionTypeInteger, Label: "Outcome ID"},
	{Name: "owner_id", Type: core.ConnectionTypeInteger, Label: "Owner ID"},
	{Name: "start_date", Type: core.ConnectionTypeString, Label: "Starts At", Placeholder: "2026-10-21T15:00:00Z"},
	{Name: "end_date", Type: core.ConnectionTypeString, Label: "Ends At", Placeholder: "2026-10-21T15:30:00Z"},
	{Name: "notes", Type: core.ConnectionTypeString, Label: "Notes"},
	{Name: "targetable_id", Type: core.ConnectionTypeString, Label: "Record ID", Placeholder: "12345", Required: true},
	{Name: "targetable_type", Type: core.ConnectionTypeString, Label: "Record Type", Placeholder: "Contact, SalesAccount or Deal", Required: true, Options: []core.ConnectionOption{{Name: "Contact", Value: "Contact"}, {Name: "Sales Account", Value: "SalesAccount"}, {Name: "Deal", Value: "Deal"}}},
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
	freshsales_common.SetString(record, "title", "title", inputs)
	freshsales_common.SetInt(record, "sales_activity_type_id", "sales_activity_type_id", inputs)
	freshsales_common.SetInt(record, "sales_activity_outcome_id", "sales_activity_outcome_id", inputs)
	freshsales_common.SetInt(record, "owner_id", "owner_id", inputs)
	freshsales_common.SetString(record, "start_date", "start_date", inputs)
	freshsales_common.SetString(record, "end_date", "end_date", inputs)
	freshsales_common.SetString(record, "notes", "notes", inputs)
	freshsales_common.SetString(record, "targetable_id", "targetable_id", inputs)
	freshsales_common.SetString(record, "targetable_type", "targetable_type", inputs)
	extra, err := freshsales_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	freshsales_common.MergeFields(record, extra)
	payload := map[string]interface{}{"sales_activity": record}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPost, "/sales_activities", payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "sales_activity")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Created sales activity %s", freshsales_common.NameOf(recordOut))), nil
}
