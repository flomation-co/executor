package airtable_record_upsert

import (
	"fmt"

	core "flomation.app/automate/executor"
	airtable "flomation.app/automate/executor/actions/airtable"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Record: Create or Update"
	Description  = "Upsert a record: Airtable updates the record whose Match Fields equal the given values, or creates a new one if none match. Returns the record and whether it was created."
	Website      = "https://www.flomation.co"
	Icon         = "airtable+rotate"
	Date         = "01/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Airtable Personal Access Token", Placeholder: "pat...", Required: true},
	{Name: "base_id", Type: core.ConnectionTypeString, Label: "Base ID", Placeholder: "appXXXXXXXXXXXXXX", Required: true},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table (ID or name)", Placeholder: "tblXXXXXXXXXXXXXX or Table 1", Required: true},
	{Name: "match_fields", Type: core.ConnectionTypeString, Label: "Match Fields", Placeholder: "Comma-separated field names to match on, e.g. Email", Required: true},
	{Name: "fields_kv", Type: core.ConnectionTypeKeyValueArray, Label: "Fields", Placeholder: "Field name = value (must include the Match Fields)"},
	{Name: "fields", Type: core.ConnectionTypeObject, Label: "Fields (JSON, advanced)", Placeholder: `{"Email":"a@b.com","Name":"Ada"}`},
	{Name: "typecast", Type: core.ConnectionTypeBoolean, Label: "Typecast", Placeholder: "Coerce string values to the field's type / create missing select options"},
}

var Outputs = [...]core.Connection{
	{Name: "id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "created", Type: core.ConnectionTypeBoolean, Label: "Created"},
	{Name: "fields", Type: core.ConnectionTypeObject, Label: "Fields"},
	{Name: "record", Type: core.ConnectionTypeObject, Label: "Record"},
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
	table, err := airtable.RequiredString("table", inputs)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}

	matchRaw, err := airtable.RequiredString("match_fields", inputs)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}
	mergeOn := airtable.CSVToList(matchRaw)
	if len(mergeOn) == 0 {
		return airtable.ErrorResult("match_fields must list at least one field name"), nil
	}

	fields, err := airtable.BuildFields(inputs)
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}
	if len(fields) == 0 {
		return airtable.ErrorResult("at least one field is required"), nil
	}
	// Each match field must be present in the record's fields, or Airtable
	// rejects the upsert — surface that clearly before the round-trip.
	for _, m := range mergeOn {
		if _, ok := fields[m]; !ok {
			return airtable.ErrorResult(fmt.Sprintf("match field %q must also be provided in Fields", m)), nil
		}
	}

	resp, err := airtable.UpsertRecord(token, baseID, table, fields, mergeOn, airtable.OptionalBool("typecast", inputs))
	if err != nil {
		return airtable.ErrorResult(err.Error()), nil
	}

	records, _ := resp["records"].([]interface{})
	if len(records) == 0 {
		return airtable.ErrorResult("Airtable upsert returned no record"), nil
	}
	rec, _ := records[0].(map[string]interface{})
	id, _ := rec["id"].(string)

	created := false
	if createdIDs, ok := resp["createdRecords"].([]interface{}); ok {
		for _, c := range createdIDs {
			if s, ok := c.(string); ok && s == id {
				created = true
				break
			}
		}
	}

	verb := "Updated"
	if created {
		verb = "Created"
	}
	return map[string]interface{}{
		"id":          id,
		"created":     created,
		"fields":      rec["fields"],
		"record":      rec,
		"result":      resp,
		"tool_result": fmt.Sprintf("%s record %s", verb, id),
		"success":     true,
		"error":       "",
	}, nil
}
