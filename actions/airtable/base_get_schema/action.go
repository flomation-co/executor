package airtable_base_get_schema

import (
	"fmt"

	core "flomation.app/automate/executor"
	airtable "flomation.app/automate/executor/actions/airtable"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Base: Get Schema"
	Description  = "Get the schema of a base — its tables, and each table's fields (name, type, options) and views. Requires the schema.bases:read scope. Returns the tables array."
	Website      = "https://www.flomation.co"
	Icon         = "airtable+table"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Airtable Personal Access Token", Placeholder: "pat...", Required: true},
	{Name: "base_id", Type: core.ConnectionTypeString, Label: "Base ID", Placeholder: "appXXXXXXXXXXXXXX", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tables", Type: core.ConnectionTypeObject, Label: "Tables"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "result", Type: core.ConnectionTypeObject, Label: "Raw Response"},
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

	resp, err := airtable.GetBaseSchema(token, baseID)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}

	tables, _ := resp["tables"].([]interface{})
	return map[string]interface{}{
		"tables":      tables,
		"count":       len(tables),
		"result":      resp,
		"tool_result": fmt.Sprintf("Base %s has %d table(s)", baseID, len(tables)),
		"success":     true,
		"error":       "",
	}, nil
}
