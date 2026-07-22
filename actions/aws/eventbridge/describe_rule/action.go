// Package aws_eventbridge_describe_rule describes an EventBridge rule.
package aws_eventbridge_describe_rule

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
	Name         = "AWS EventBridge Describe Rule"
	Description  = "Fetch the details of an EventBridge rule by name."
	Website      = "https://www.flomation.co"
	Icon         = "bolt+magnifying-glass"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Rule Name", Placeholder: "my-rule", Required: true},
	{Name: "event_bus_name", Type: core.ConnectionTypeString, Label: "Event Bus Name (optional)", Placeholder: "default"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "Rule ARN"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "event_pattern", Type: core.ConnectionTypeString, Label: "Event Pattern"},
	{Name: "schedule_expression", Type: core.ConnectionTypeString, Label: "Schedule Expression"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("name", inputs))
	if name == "" {
		return nil, fmt.Errorf("rule name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.DescribeRuleInput{Name: aws.String(name)}
	if bus := strings.TrimSpace(awscommon.InputString("event_bus_name", inputs)); bus != "" {
		in.EventBusName = aws.String(bus)
	}

	out, err := client.DescribeRule(ctx, in)
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.Arn)
	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Rule %s is %s", name, string(out.State)),
		"arn":                 arn,
		"state":               string(out.State),
		"event_pattern":       aws.ToString(out.EventPattern),
		"schedule_expression": aws.ToString(out.ScheduleExpression),
	}, nil
}
