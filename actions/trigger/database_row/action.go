// Package database_row declares the "new database row" trigger.
//
// Like every trigger in this repo (s3, git_poll, airtable_poll, schedule, ...),
// the executor half is purely declarative: Inputs are the poll configuration the
// Launch poller reads, Outputs are the row schema Launch populates when it fires
// the flow, and Execute simply echoes any injected data. The actual polling loop
// — connecting to the database on an interval, tracking a monotonic cursor column
// (an auto-increment id or a created_at/updated_at timestamp), and invoking the
// flow once per newly-inserted row — lives in the Launch service
// (internal/dbrow), mirroring the S3 trigger.
//
// When a new row is detected, the fired trigger data carries every column of the
// row as a top-level key (so `${email}` resolves the row's email column), plus
// the whole `row` object and some metadata. See Outputs below.
package database_row

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Database Row Trigger"
	Description  = "Triggers a flow when a new row is inserted into a SQL table. Polls on an interval, tracking a monotonic cursor column (id or timestamp)."
	Website      = "https://www.flomation.co"
	Icon         = "database+plus"
	Date         = "16/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "dialect", Type: core.ConnectionTypeString, Label: "Database Type", Required: true, Options: []core.ConnectionOption{
		{Name: "PostgreSQL", Value: "postgresql"},
		{Name: "MySQL / MariaDB", Value: "mysql"},
		{Name: "SQL Server", Value: "sqlserver"},
	}},
	{Name: "host", Type: core.ConnectionTypeString, Label: "Database Host", Placeholder: "localhost", Required: true},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Database Port", Placeholder: "5432 / 3306 / 1433", Required: true},
	{Name: "username", Type: core.ConnectionTypeString, Label: "Username", Required: true},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password", Required: true},
	{Name: "database", Type: core.ConnectionTypeString, Label: "Database Name", Required: true},
	{Name: "ssl_mode", Type: core.ConnectionTypeString, Label: "SSL / TLS Mode", Placeholder: "Connection encryption", Options: []core.ConnectionOption{
		{Name: "Disable", Value: "disable"},
		{Name: "Require", Value: "require"},
		{Name: "Verify (full)", Value: "verify"},
	}},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table", Placeholder: "orders (optionally schema.orders)", Required: true},
	{Name: "cursor_column", Type: core.ConnectionTypeString, Label: "Cursor Column", Placeholder: "id or created_at — must only ever increase", Required: true},
	{Name: "poll_interval", Type: core.ConnectionTypeString, Label: "Poll Interval", Placeholder: "e.g. 60s, 5m"},
	{Name: "filter_column", Type: core.ConnectionTypeString, Label: "Filter Column", Placeholder: "Only fire when this column equals the value below (optional)"},
	{Name: "filter_value", Type: core.ConnectionTypeString, Label: "Filter Value", Placeholder: "e.g. paid, active (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "row", Type: core.ConnectionTypeObject, Label: "Row"},
	{Name: "cursor", Type: core.ConnectionTypeString, Label: "Cursor Value"},
	{Name: "table", Type: core.ConnectionTypeString, Label: "Table"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// Execute echoes any injected configuration. Poll-based triggers do no work
// inside the executor — Launch drives the polling loop and populates the outputs.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}
	return result, nil
}
