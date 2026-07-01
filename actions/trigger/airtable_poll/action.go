// Package airtable_poll declares the Airtable "new/updated record" trigger.
//
// Like every trigger in this repo (git_poll, linkedin_poll, schedule, ...),
// the executor half is declarative: Inputs are the poll configuration the API
// poller reads, Outputs are the record schema the API populates when it fires
// the flow, and Execute simply echoes any injected data. The actual polling
// loop — calling Airtable on an interval, tracking the last-modified cursor,
// and invoking the executor per changed record — lives in the API repo and is
// a separate follow-up.
package airtable_poll

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "Airtable Trigger"
	Description  = "Triggers a flow when records are created or updated in an Airtable table. Polls the table on an interval using a Created Time or Last Modified Time field."
	Website      = "https://www.flomation.co"
	Icon         = "airtable"
	Date         = "01/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "access_token", Type: core.ConnectionTypeSecret, Label: "Airtable Personal Access Token", Placeholder: "pat...", Required: true},
	{Name: "base_id", Type: core.ConnectionTypeString, Label: "Base ID", Placeholder: "appXXXXXXXXXXXXXX", Required: true},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table (ID or name)", Placeholder: "tblXXXXXXXXXXXXXX or Table 1", Required: true},
	{Name: "trigger_field", Type: core.ConnectionTypeString, Label: "Trigger Field", Placeholder: "A 'Created Time' or 'Last Modified Time' field name", Required: true},
	{Name: "trigger_on", Type: core.ConnectionTypeString, Label: "Trigger On", Options: []core.ConnectionOption{
		{Name: "Record created", Value: "created"},
		{Name: "Record updated", Value: "updated"},
		{Name: "Record created or updated", Value: "created_or_updated"},
	}},
	{Name: "view", Type: core.ConnectionTypeString, Label: "View", Placeholder: "Restrict to a view name or ID (optional)"},
	{Name: "formula", Type: core.ConnectionTypeText, Label: "Filter by Formula", Placeholder: "Extra Airtable formula, ANDed with the timestamp filter (optional)"},
	{Name: "poll_interval", Type: core.ConnectionTypeString, Label: "Poll Interval", Placeholder: "e.g. 1m, 5m"},
	{Name: "download_attachments", Type: core.ConnectionTypeBoolean, Label: "Download Attachments", Placeholder: "Download attachment fields as binary"},
	{Name: "download_fields", Type: core.ConnectionTypeString, Label: "Attachment Fields", Placeholder: "Comma-separated attachment field names (case sensitive)"},
}

var Outputs = [...]core.Connection{
	{Name: "record_id", Type: core.ConnectionTypeString, Label: "Record ID"},
	{Name: "created_time", Type: core.ConnectionTypeString, Label: "Created Time"},
	{Name: "fields", Type: core.ConnectionTypeObject, Label: "Fields"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}
	return result, nil
}
