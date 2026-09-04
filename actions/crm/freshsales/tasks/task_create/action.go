// Package task_create implements the Freshsales "Task: Create" action.
package task_create

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
	Name         = "Task: Create"
	Description  = "Create a Freshsales task."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+plus"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "title", Type: core.ConnectionTypeString, Label: "Title", Placeholder: "Call Ada about the rollout", Required: true},
	{Name: "due_date", Type: core.ConnectionTypeString, Label: "Due Date", Placeholder: "2026-10-21T15:00:00Z", Required: true},
	{Name: "owner_id", Type: core.ConnectionTypeInteger, Label: "Owner ID"},
	{Name: "creater_id", Type: core.ConnectionTypeInteger, Label: "Creator ID"},
	{Name: "outcome_id", Type: core.ConnectionTypeString, Label: "Outcome ID"},
	{Name: "task_type_id", Type: core.ConnectionTypeInteger, Label: "Task Type ID"},
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
	freshsales_common.SetString(record, "due_date", "due_date", inputs)
	freshsales_common.SetInt(record, "owner_id", "owner_id", inputs)
	freshsales_common.SetInt(record, "creater_id", "creater_id", inputs)
	freshsales_common.SetString(record, "outcome_id", "outcome_id", inputs)
	freshsales_common.SetInt(record, "task_type_id", "task_type_id", inputs)
	freshsales_common.SetString(record, "targetable_id", "targetable_id", inputs)
	freshsales_common.SetString(record, "targetable_type", "targetable_type", inputs)
	extra, err := freshsales_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	freshsales_common.MergeFields(record, extra)
	payload := map[string]interface{}{"task": record}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPost, "/tasks", payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "task")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Created task %s", freshsales_common.NameOf(recordOut))), nil
}
