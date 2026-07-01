package airtable_record_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	airtable "flomation.app/automate/executor/actions/airtable"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Record: Update"
	Description  = "Update fields on an existing Airtable record by its record ID. Only the supplied fields change. Returns the updated record."
	Website      = "https://www.flomation.co"
	Icon         = "airtable+pencil"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Airtable Personal Access Token", Placeholder: "pat...", Required: true},
	{Name: "base_id", Type: core.ConnectionTypeString, Label: "Base ID", Placeholder: "appXXXXXXXXXXXXXX", Required: true},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table (ID or name)", Placeholder: "tblXXXXXXXXXXXXXX or Table 1", Required: true},
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record ID", Placeholder: "recXXXXXXXXXXXXXX", Required: true},
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
	recordID, err := airtable.RequiredString("record_id", inputs)
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

	rec, err := airtable.UpdateRecord(token, baseID, table, recordID, fields, airtable.OptionalBool("typecast", inputs))
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}

	return airtable.RecordResult(rec, fmt.Sprintf("Updated record %s", recordID)), nil
}
