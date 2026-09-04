// Package task_update implements the Freshsales "Task: Update" action.
package task_update

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
	Name         = "Task: Update"
	Description  = "Update an existing Freshsales task."
	Website      = "https://www.flomation.co"
	Icon         = "freshworks+pencil"
	Date         = "04/09/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "api_key", Type: core.ConnectionTypeSecret, Label: "Freshsales API Key", Placeholder: "${secrets.FreshsalesApiKey}", Required: true},
	{Name: "account", Type: core.ConnectionTypeString, Label: "Account (bundle alias, e.g. widgetz)", Placeholder: "widgetz", Required: true},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Task ID", Placeholder: "12345", Required: true},
	{Name: "owner_id", Type: core.ConnectionTypeInteger, Label: "Owner ID"},
	{Name: "creater_id", Type: core.ConnectionTypeInteger, Label: "Creator ID"},
	{Name: "outcome_id", Type: core.ConnectionTypeString, Label: "Outcome ID"},
	{Name: "task_type_id", Type: core.ConnectionTypeInteger, Label: "Task Type ID"},
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

	idValue, err := freshsales_common.RequiredString("id", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	record := map[string]interface{}{}
	freshsales_common.SetInt(record, "owner_id", "owner_id", inputs)
	freshsales_common.SetInt(record, "creater_id", "creater_id", inputs)
	freshsales_common.SetString(record, "outcome_id", "outcome_id", inputs)
	freshsales_common.SetInt(record, "task_type_id", "task_type_id", inputs)
	extra, err := freshsales_common.ParseJSONObject("fields", inputs)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}
	freshsales_common.MergeFields(record, extra)
	payload := map[string]interface{}{"task": record}

	var query url.Values

	resp, err := client.Do(flow, http.MethodPut, "/tasks/"+idValue, payload, query)
	if err != nil {
		return freshsales_common.ErrorResult(err.Error()), nil
	}

	recordOut := freshsales_common.Obj(resp, "task")
	if recordOut == nil {
		recordOut = resp
	}
	return freshsales_common.ObjectResult(recordOut, fmt.Sprintf("Updated task %s", freshsales_common.NameOf(recordOut))), nil
}
