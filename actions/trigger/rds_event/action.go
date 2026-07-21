// Package rds_event declares the "RDS event" trigger.
//
// Like every trigger in this repo (s3, database_row, git_poll, ...), the executor
// half is purely declarative: Inputs are the poll configuration the Launch poller
// reads, Outputs are the event schema Launch populates when it fires the flow, and
// Execute simply echoes any injected data. The polling loop — calling RDS
// DescribeEvents on an interval and firing the flow once per newly-observed event
// — lives in the Launch service (internal/rdsevent), mirroring the S3 trigger.
//
// When a new event is detected, the fired trigger data carries the event fields as
// top-level keys (so `${message}` / `${source_identifier}` resolve directly), plus
// metadata. See Outputs below.
package rds_event

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "RDS Event Trigger"
	Description  = "Triggers a flow on AWS RDS/Aurora events (failover, backup complete, low storage, availability change). Polls DescribeEvents on an interval."
	Website      = "https://www.flomation.co"
	Icon         = "database+bell"
	Date         = "21/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "source_type", Type: core.ConnectionTypeString, Label: "Source Type", Placeholder: "Any", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "DB Instance", Value: "db-instance"},
		{Name: "DB Cluster", Value: "db-cluster"},
		{Name: "DB Snapshot", Value: "db-snapshot"},
		{Name: "DB Cluster Snapshot", Value: "db-cluster-snapshot"},
		{Name: "DB Parameter Group", Value: "db-parameter-group"},
		{Name: "DB Security Group", Value: "db-security-group"},
	}},
	{Name: "source_identifier", Type: core.ConnectionTypeString, Label: "Source Identifier", Placeholder: "Only this instance/cluster (optional; requires a Source Type)"},
	{Name: "event_categories", Type: core.ConnectionTypeString, Label: "Event Categories", Placeholder: "Comma-separated filter, e.g. failover,availability (optional)"},
	{Name: "poll_interval", Type: core.ConnectionTypeString, Label: "Poll Interval", Placeholder: "e.g. 60s, 5m"},
}

var Outputs = [...]core.Connection{
	{Name: "source_identifier", Type: core.ConnectionTypeString, Label: "Source Identifier"},
	{Name: "source_type", Type: core.ConnectionTypeString, Label: "Source Type"},
	{Name: "source_arn", Type: core.ConnectionTypeString, Label: "Source ARN"},
	{Name: "message", Type: core.ConnectionTypeString, Label: "Message"},
	{Name: "event_categories", Type: core.ConnectionTypeString, Label: "Event Categories"},
	{Name: "date", Type: core.ConnectionTypeString, Label: "Event Time"},
	{Name: "triggered_at", Type: core.ConnectionTypeString, Label: "Triggered At"},
}

// Execute echoes any injected configuration. Poll-based triggers do no work inside
// the executor — Launch drives the polling loop and populates the outputs.
func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	for _, input := range inputs {
		if input.Value != nil {
			result[input.Name] = input.Value
		}
	}
	return result, nil
}
