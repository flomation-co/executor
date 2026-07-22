// Package aws_cloudwatch_set_alarm_state temporarily sets the state of a
// CloudWatch alarm (useful for testing alarm actions).
package aws_cloudwatch_set_alarm_state

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	cwtypes "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Set Alarm State"
	Description  = "Temporarily set an alarm's state to test its actions."
	Website      = "https://www.flomation.co"
	Icon         = "bell+pen"
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
	{Name: "alarm_name", Type: core.ConnectionTypeString, Label: "Alarm Name", Placeholder: "high-cpu", Required: true},
	{Name: "state_value", Type: core.ConnectionTypeString, Label: "State", Required: true, Options: []core.ConnectionOption{
		{Name: "OK", Value: "OK"},
		{Name: "ALARM", Value: "ALARM"},
		{Name: "INSUFFICIENT_DATA", Value: "INSUFFICIENT_DATA"},
	}},
	{Name: "state_reason", Type: core.ConnectionTypeString, Label: "State Reason", Placeholder: "Manual test", Required: true},
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
	stateValue := strings.TrimSpace(awscommon.InputString("state_value", inputs))
	if stateValue == "" {
		return nil, fmt.Errorf("state_value is required")
	}
	stateReason := strings.TrimSpace(awscommon.InputString("state_reason", inputs))
	if stateReason == "" {
		return nil, fmt.Errorf("state_reason is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatch.NewFromConfig(cfg)

	_, err = client.SetAlarmState(ctx, &cloudwatch.SetAlarmStateInput{
		AlarmName:   aws.String(alarmName),
		StateValue:  cwtypes.StateValue(stateValue),
		StateReason: aws.String(stateReason),
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Set alarm %s to state %s", alarmName, stateValue),
		"alarm_name":  alarmName,
	}, nil
}
