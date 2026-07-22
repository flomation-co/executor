// Package cloudwatch_logs declares the "CloudWatch Logs Pattern" trigger.
//
// The executor half is purely declarative (see cloudwatch_alarm). The Launch
// poller (internal/cloudwatch) calls FilterLogEvents on an interval over a rolling
// time window, and fires the flow once per newly-observed log event that matches
// the filter pattern. Events are deduped by their CloudWatch event id and a
// timestamp watermark in trigger_state, so an event never fires twice across
// overlapping poll windows.
package cloudwatch_logs

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "CloudWatch Logs Trigger"
	Description  = "Triggers a flow when log events match a filter pattern in a log group. Polls FilterLogEvents."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+bell"
	Date         = "22/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "log_group_name", Type: core.ConnectionTypeString, Label: "Log Group Name", Placeholder: "/aws/lambda/my-fn", Required: true},
	{Name: "filter_pattern", Type: core.ConnectionTypeString, Label: "Filter Pattern", Placeholder: "e.g. ERROR (blank matches all events)"},
	{Name: "poll_interval", Type: core.ConnectionTypeString, Label: "Poll Interval", Placeholder: "e.g. 60s, 5m"},
}

var Outputs = [...]core.Connection{
	{Name: "log_group", Type: core.ConnectionTypeString, Label: "Log Group"},
	{Name: "log_stream", Type: core.ConnectionTypeString, Label: "Log Stream"},
	{Name: "message", Type: core.ConnectionTypeString, Label: "Message"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Event Time"},
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
