// Package aws_cloudwatch_put_composite_alarm creates or updates a CloudWatch
// composite alarm from a rule expression over other alarms.
package aws_cloudwatch_put_composite_alarm

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Put Composite Alarm"
	Description  = "Create or update a composite alarm from a rule over other alarms."
	Website      = "https://www.flomation.co"
	Icon         = "bell+plus"
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
	{Name: "alarm_name", Type: core.ConnectionTypeString, Label: "Alarm Name", Placeholder: "service-degraded", Required: true},
	{Name: "alarm_rule", Type: core.ConnectionTypeString, Label: "Alarm Rule", Placeholder: `ALARM("high-cpu") OR ALARM("high-mem")`, Required: true},
	{Name: "alarm_actions", Type: core.ConnectionTypeString, Label: "Alarm Actions (comma-separated ARNs)", Placeholder: "arn:aws:sns:eu-west-2:123456789012:alerts"},
	{Name: "alarm_description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "alarm_name", Type: core.ConnectionTypeString, Label: "Alarm Name"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	alarmName := strings.TrimSpace(awscommon.InputString("alarm_name", inputs))
	if alarmName == "" {
		return nil, fmt.Errorf("alarm_name is required")
	}
	alarmRule := strings.TrimSpace(awscommon.InputString("alarm_rule", inputs))
	if alarmRule == "" {
		return nil, fmt.Errorf("alarm_rule is required")
	}

	in := &cloudwatch.PutCompositeAlarmInput{
		AlarmName: aws.String(alarmName),
		AlarmRule: aws.String(alarmRule),
	}
	if actions := splitCSV(awscommon.InputString("alarm_actions", inputs)); len(actions) > 0 {
		in.AlarmActions = actions
	}
	if desc := strings.TrimSpace(awscommon.InputString("alarm_description", inputs)); desc != "" {
		in.AlarmDescription = aws.String(desc)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	if _, err := client.PutCompositeAlarm(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Composite alarm %s created/updated", alarmName),
		"alarm_name":  alarmName,
	}, nil
}

func splitCSV(raw string) []string {
	var out []string
	for _, s := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}
