// Package cloudwatch_metric declares the "CloudWatch Metric Threshold" trigger.
//
// The executor half is purely declarative (see cloudwatch_alarm). The Launch
// poller (internal/cloudwatch) calls GetMetricStatistics on an interval, compares
// the latest datapoint against the configured threshold, and fires the flow when
// the metric *enters* breach (edge-triggered — it does not re-fire every poll
// while the metric stays breached). This is a lighter-weight alternative to a
// full CloudWatch alarm when you just want a flow to run on a threshold crossing.
package cloudwatch_metric

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "CloudWatch Metric Threshold Trigger"
	Description  = "Triggers a flow when a CloudWatch metric crosses a threshold (edge-triggered). Polls GetMetricStatistics."
	Website      = "https://www.flomation.co"
	Icon         = "gauge+bell"
	Date         = "22/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "e.g. AWS/EC2", Required: true},
	{Name: "metric_name", Type: core.ConnectionTypeString, Label: "Metric Name", Placeholder: "e.g. CPUUtilization", Required: true},
	{Name: "dimensions", Type: core.ConnectionTypeString, Label: "Dimensions", Placeholder: "Name=Value,Name2=Value2 (optional)"},
	{Name: "statistic", Type: core.ConnectionTypeString, Label: "Statistic", Required: true, Options: []core.ConnectionOption{
		{Name: "Average", Value: "Average"},
		{Name: "Sum", Value: "Sum"},
		{Name: "Minimum", Value: "Minimum"},
		{Name: "Maximum", Value: "Maximum"},
		{Name: "Sample Count", Value: "SampleCount"},
	}},
	{Name: "period", Type: core.ConnectionTypeInteger, Label: "Period (seconds)", Placeholder: "e.g. 300"},
	{Name: "comparison", Type: core.ConnectionTypeString, Label: "Comparison", Required: true, Options: []core.ConnectionOption{
		{Name: "Greater than", Value: "GreaterThanThreshold"},
		{Name: "Greater than or equal", Value: "GreaterThanOrEqualToThreshold"},
		{Name: "Less than", Value: "LessThanThreshold"},
		{Name: "Less than or equal", Value: "LessThanOrEqualToThreshold"},
	}},
	{Name: "threshold", Type: core.ConnectionTypeString, Label: "Threshold", Placeholder: "e.g. 80", Required: true},
	{Name: "poll_interval", Type: core.ConnectionTypeString, Label: "Poll Interval", Placeholder: "e.g. 60s, 5m"},
}

var Outputs = [...]core.Connection{
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace"},
	{Name: "metric_name", Type: core.ConnectionTypeString, Label: "Metric Name"},
	{Name: "value", Type: core.ConnectionTypeString, Label: "Value"},
	{Name: "threshold", Type: core.ConnectionTypeString, Label: "Threshold"},
	{Name: "comparison", Type: core.ConnectionTypeString, Label: "Comparison"},
	{Name: "statistic", Type: core.ConnectionTypeString, Label: "Statistic"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Datapoint Time"},
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
