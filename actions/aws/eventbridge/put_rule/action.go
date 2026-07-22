// Package aws_eventbridge_put_rule creates or updates an EventBridge rule.
package aws_eventbridge_put_rule

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EventBridge Put Rule"
	Description  = "Create or update an EventBridge rule from an event pattern or schedule."
	Website      = "https://www.flomation.co"
	Icon         = "bolt+plus"
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
	{Name: "event_pattern", Type: core.ConnectionTypeString, Label: "Event Pattern (JSON)", Placeholder: `{"source":["aws.ec2"]}`},
	{Name: "schedule_expression", Type: core.ConnectionTypeString, Label: "Schedule Expression", Placeholder: "rate(5 minutes) or cron(0 20 * * ? *)"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State", Options: []core.ConnectionOption{
		{Name: "Enabled", Value: "ENABLED"},
		{Name: "Disabled", Value: "DISABLED"},
	}},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
	{Name: "event_bus_name", Type: core.ConnectionTypeString, Label: "Event Bus Name (optional)", Placeholder: "default"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rule_arn", Type: core.ConnectionTypeString, Label: "Rule ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("name", inputs))
	if name == "" {
		return nil, fmt.Errorf("rule name is required")
	}
	eventPattern := strings.TrimSpace(awscommon.InputString("event_pattern", inputs))
	scheduleExpression := strings.TrimSpace(awscommon.InputString("schedule_expression", inputs))
	if eventPattern == "" && scheduleExpression == "" {
		return nil, fmt.Errorf("one of event_pattern or schedule_expression is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.PutRuleInput{Name: aws.String(name)}
	if eventPattern != "" {
		in.EventPattern = aws.String(eventPattern)
	}
	if scheduleExpression != "" {
		in.ScheduleExpression = aws.String(scheduleExpression)
	}
	if state := strings.TrimSpace(awscommon.InputString("state", inputs)); state != "" {
		in.State = ebtypes.RuleState(state)
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if bus := strings.TrimSpace(awscommon.InputString("event_bus_name", inputs)); bus != "" {
		in.EventBusName = aws.String(bus)
	}

	out, err := client.PutRule(ctx, in)
	if err != nil {
		return nil, err
	}

	ruleArn := aws.ToString(out.RuleArn)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Put rule %s (%s)", name, ruleArn),
		"rule_arn":    ruleArn,
	}, nil
}
