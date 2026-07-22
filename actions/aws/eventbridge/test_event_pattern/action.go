// Package aws_eventbridge_test_event_pattern tests whether a sample event
// matches an EventBridge event pattern.
package aws_eventbridge_test_event_pattern

import (
	"context"
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
	Name         = "AWS EventBridge Test Event Pattern"
	Description  = "Test whether a sample event matches an EventBridge event pattern."
	Website      = "https://www.flomation.co"
	Icon         = "bolt+circle-check"
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
	{Name: "event_pattern", Type: core.ConnectionTypeString, Label: "Event Pattern (JSON)", Placeholder: `{"source":["aws.ec2"]}`, Required: true},
	{Name: "event", Type: core.ConnectionTypeString, Label: "Sample Event (JSON)", Placeholder: `{"source":"aws.ec2","detail-type":"..."}`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "matches", Type: core.ConnectionTypeBoolean, Label: "Matches"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	pattern := strings.TrimSpace(awscommon.InputString("event_pattern", inputs))
	if pattern == "" {
		return nil, fmt.Errorf("event_pattern is required")
	}
	event := strings.TrimSpace(awscommon.InputString("event", inputs))
	if event == "" {
		return nil, fmt.Errorf("event is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	out, err := client.TestEventPattern(ctx, &eventbridge.TestEventPatternInput{
		EventPattern: aws.String(pattern),
		Event:        aws.String(event),
	})
	if err != nil {
		return nil, err
	}

	summary := "Event does not match the pattern"
	if out.Result {
		summary = "Event matches the pattern"
	}
	return map[string]interface{}{
		"tool_result": summary,
		"matches":     out.Result,
	}, nil
}
