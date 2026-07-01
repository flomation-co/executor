package airtable_record_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	airtable "flomation.app/automate/executor/actions/airtable"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Record: Delete"
	Description  = "Delete a record from an Airtable table by its record ID. Returns the deleted record ID."
	Website      = "https://www.flomation.co"
	Icon         = "airtable+trash"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Airtable Personal Access Token", Placeholder: "pat...", Required: true},
	{Name: "base_id", Type: core.ConnectionTypeString, Label: "Base ID", Placeholder: "appXXXXXXXXXXXXXX", Required: true},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table (ID or name)", Placeholder: "tblXXXXXXXXXXXXXX or Table 1", Required: true},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record ID", Placeholder: "recXXXXXXXXXXXXXX", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Deleted Record ID"},
	{Name: "deleted", Type: core.ConnectionTypeBoolean, Label: "Deleted"},
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result Summary"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	token, err := airtable.GetAccessToken(inputs)
	if err != nil {
		return nil, err
	}
	baseID, err := airtable.RequiredString("base_id", inputs)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}
	table, err := airtable.RequiredString("table", inputs)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}
	recordID, err := airtable.RequiredString("record_id", inputs)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}

	resp, err := airtable.DeleteRecord(token, baseID, table, recordID)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}

	id, _ := resp["id"].(string)
	deleted, _ := resp["deleted"].(bool)
	return map[string]interface{}{
		"id":          id,
		"deleted":     deleted,
		"tool_result": fmt.Sprintf("Deleted record %s", recordID),
		"success":     true,
		"error":       "",
	}, nil
}
