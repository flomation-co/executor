// Package cloudwatch_alarm declares the "CloudWatch Alarm" trigger.
//
// Like every trigger in this repo (rds_event, s3, ...), the executor half is
// purely declarative: Inputs are the poll configuration the Launch poller reads,
// Outputs are the schema Launch populates when it fires, and Execute simply
// echoes injected data. The polling loop — calling CloudWatch DescribeAlarms on
// an interval and firing the flow once per state transition — lives in the Launch
// service (internal/cloudwatch).
//
// This is the trigger that closes the alarms→scaling loop: an alarm entering
// ALARM can start a flow that scales an Auto Scaling group, notifies a channel,
// or runs any remediation.
package cloudwatch_alarm

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "CloudWatch Alarm Trigger"
	Description  = "Triggers a flow when a CloudWatch alarm changes state (OK, ALARM, INSUFFICIENT_DATA). Polls DescribeAlarms."
	Website      = "https://www.flomation.co"
	Icon         = "bell+bolt"
	Date         = "22/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "alarm_names", Type: core.ConnectionTypeString, Label: "Alarm Names", Placeholder: "Comma-separated; leave blank for all alarms (optional)"},
	{Name: "alarm_state", Type: core.ConnectionTypeString, Label: "Fire On State", Placeholder: "Any", Options: []core.ConnectionOption{
		{Name: "Any state change", Value: ""},
		{Name: "ALARM", Value: "ALARM"},
		{Name: "OK", Value: "OK"},
		{Name: "INSUFFICIENT_DATA", Value: "INSUFFICIENT_DATA"},
	}},
	{Name: "poll_interval", Type: core.ConnectionTypeString, Label: "Poll Interval", Placeholder: "e.g. 60s, 5m"},
}

var Outputs = [...]core.Connection{
	{Name: "alarm_name", Type: core.ConnectionTypeString, Label: "Alarm Name"},
	{Name: "state_value", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "previous_state", Type: core.ConnectionTypeString, Label: "Previous State"},
	{Name: "state_reason", Type: core.ConnectionTypeString, Label: "State Reason"},
	{Name: "metric_name", Type: core.ConnectionTypeString, Label: "Metric Name"},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "State Changed At"},
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
