package airtable_record_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	airtable "flomation.app/automate/executor/actions/airtable"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Record: Create"
	Description  = "Create a record in an Airtable table. Set fields as simple key/value rows or as a JSON object for typed values (arrays, linked records, attachments). Returns the new record and its ID."
	Website      = "https://www.flomation.co"
	Icon         = "airtable+plus"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Airtable Personal Access Token", Placeholder: "pat...", Required: true},
	{Name: "base_id", Type: core.ConnectionTypeString, Label: "Base ID", Placeholder: "appXXXXXXXXXXXXXX", Required: true},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table (ID or name)", Placeholder: "tblXXXXXXXXXXXXXX or Table 1", Required: true},
	{Name: "fields_kv", Type: core.ConnectionTypeKeyValueArray, Label: "Fields", Placeholder: "Field name = value (for simple text/number/single-select fields)"},
	{Name: "fields", Type: core.ConnectionTypeObject, Label: "Fields (JSON, advanced)", Placeholder: `{"Name":"Ada","Tags":["A","B"],"Done":true}`},
	{Name: "typecast", Type: core.ConnectionTypeBoolean, Label: "Typecast", Placeholder: "Coerce string values to the field's type / create missing select options"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "fields", Type: core.ConnectionTypeObject, Label: "Fields"},
	{Name: "record", Type: core.ConnectionTypeObject, Label: "Record"},
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

	fields, err := airtable.BuildFields(inputs)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}
	if len(fields) == 0 {
		return airtable.ErrorResult("at least one field is required"), nil
	}

	rec, err := airtable.CreateRecord(token, baseID, table, fields, airtable.OptionalBool("typecast", inputs))
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}

	id, _ := rec["id"].(string)
	return airtable.RecordResult(rec, fmt.Sprintf("Created record %s", id)), nil
}
