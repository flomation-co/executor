// Package aws_eventbridge_list_targets_by_rule lists the targets assigned to an
// EventBridge rule.
package aws_eventbridge_list_targets_by_rule

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EventBridge List Targets By Rule"
	Description  = "List the targets assigned to an EventBridge rule."
	Website      = "https://www.flomation.co"
	Icon         = "bolt+list"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{
		{Name: "Access Keys", Value: "keys"},
		{Name: "Assume Role (cross-account)", Value: "assume_role"},
		{Name: "Managed Role (Credential)", Value: "credential"},
	}},
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Role ARN to Assume", Placeholder: "arn:aws:iam::<your-account>:role/FlomationAccess", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "AWS Role Credential", Required: true, Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"credential"}}},
	{Name: "rule", Type: core.ConnectionTypeString, Label: "Rule Name", Placeholder: "my-rule", Required: true},
	{Name: "event_bus_name", Type: core.ConnectionTypeString, Label: "Event Bus Name (optional)", Placeholder: "default"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "targets", Type: core.ConnectionTypeString, Label: "Targets (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	rule := strings.TrimSpace(awscommon.InputString("rule", inputs))
	if rule == "" {
		return nil, fmt.Errorf("rule name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.ListTargetsByRuleInput{Rule: aws.String(rule)}
	if bus := strings.TrimSpace(awscommon.InputString("event_bus_name", inputs)); bus != "" {
		in.EventBusName = aws.String(bus)
	}

	out, err := client.ListTargetsByRule(ctx, in)
	if err != nil {
		return nil, err
	}

	type targetSummary struct {
		ID      string `json:"id"`
		Arn     string `json:"arn"`
		RoleArn string `json:"role_arn"`
	}
	summaries := make([]targetSummary, 0, len(out.Targets))
	for _, t := range out.Targets {
		summaries = append(summaries, targetSummary{
			ID:      aws.ToString(t.Id),
			Arn:     aws.ToString(t.Arn),
			RoleArn: aws.ToString(t.RoleArn),
		})
	}

	targetsJSON, err := json.Marshal(summaries)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d target(s) on rule %s", len(summaries), rule),
		"targets":     string(targetsJSON),
		"count":       len(summaries),
	}, nil
}
