// Package route53_health_check declares the "Route 53 Health Check" trigger.
//
// Like every trigger in this repo (cloudwatch_alarm, rds_event, ...), the executor
// half is purely declarative: Inputs are the poll configuration the Launch poller
// reads, Outputs are the schema Launch populates when it fires, and Execute simply
// echoes injected data. The polling loop — calling Route 53 GetHealthCheckStatus
// on an interval and firing when the aggregate health flips — lives in the Launch
// service (internal/route53health).
//
// This lets a health check drive a failover flow: when an endpoint goes unhealthy,
// start a flow that reroutes DNS, pages the team, or spins up a replacement.
package route53_health_check

import (
	core "flomation.app/automate/executor"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "Route 53 Health Check Trigger"
	Description  = "Triggers a flow when a Route 53 health check goes unhealthy (or recovers). Polls GetHealthCheckStatus."
	Website      = "https://www.flomation.co"
	Icon         = "gauge+bell"
	Date         = "22/07/2026"
	Type         = core.ActionTypeTrigger
)

var Inputs = [...]core.Connection{
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2 (ignored — Route 53 is global)"},
	{Name: "health_check_id", Type: core.ConnectionTypeString, Label: "Health Check ID", Required: true},
	{Name: "fire_on", Type: core.ConnectionTypeString, Label: "Fire On", Placeholder: "Unhealthy", Options: []core.ConnectionOption{
		{Name: "Becomes unhealthy", Value: "unhealthy"},
		{Name: "Becomes healthy", Value: "healthy"},
		{Name: "Any change", Value: ""},
	}},
	{Name: "poll_interval", Type: core.ConnectionTypeString, Label: "Poll Interval", Placeholder: "e.g. 60s, 5m"},
}

var Outputs = [...]core.Connection{
	{Name: "health_check_id", Type: core.ConnectionTypeString, Label: "Health Check ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
	{Name: "previous_status", Type: core.ConnectionTypeString, Label: "Previous Status"},
	{Name: "healthy_count", Type: core.ConnectionTypeInteger, Label: "Healthy Checkers"},
	{Name: "unhealthy_count", Type: core.ConnectionTypeInteger, Label: "Unhealthy Checkers"},
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
