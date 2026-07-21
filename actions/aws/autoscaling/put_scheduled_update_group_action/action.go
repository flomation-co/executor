// Package aws_autoscaling_put_scheduled_update_group_action schedules a capacity change.
package aws_autoscaling_put_scheduled_update_group_action

import (
	"context"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Auto Scaling Put Scheduled Action"
	Description  = "Schedule a capacity change (min/max/desired) on an Auto Scaling group."
	Website      = "https://www.flomation.co"
	Icon         = "clock+plus"
	Date         = "21/07/2026"
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
	{Name: "auto_scaling_group_name", Type: core.ConnectionTypeString, Label: "Auto Scaling Group Name", Placeholder: "my-asg", Required: true},
	{Name: "scheduled_action_name", Type: core.ConnectionTypeString, Label: "Scheduled Action Name", Placeholder: "scale-up-morning", Required: true},
	{Name: "min_size", Type: core.ConnectionTypeInteger, Label: "Min Size", Placeholder: "1"},
	{Name: "max_size", Type: core.ConnectionTypeInteger, Label: "Max Size", Placeholder: "10"},
	{Name: "desired_capacity", Type: core.ConnectionTypeInteger, Label: "Desired Capacity", Placeholder: "4"},
	{Name: "recurrence", Type: core.ConnectionTypeString, Label: "Recurrence (cron)", Placeholder: "0 9 * * *"},
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time (RFC3339)", Placeholder: "2026-08-01T09:00:00Z"},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time (RFC3339)", Placeholder: "2026-08-31T09:00:00Z"},
	{Name: "time_zone", Type: core.ConnectionTypeString, Label: "Time Zone", Placeholder: "Europe/London"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	groupName := awscommon.InputString("auto_scaling_group_name", inputs)
	if groupName == "" {
		return nil, fmt.Errorf("auto scaling group name is required")
	}
	actionName := awscommon.InputString("scheduled_action_name", inputs)
	if actionName == "" {
		return nil, fmt.Errorf("scheduled action name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := autoscaling.NewFromConfig(cfg)

	in := &autoscaling.PutScheduledUpdateGroupActionInput{
		AutoScalingGroupName: aws.String(groupName),
		ScheduledActionName:  aws.String(actionName),
	}
	if n, ok := awscommon.InputInt("min_size", inputs); ok {
		in.MinSize = aws.Int32(int32(n))
	}
	if n, ok := awscommon.InputInt("max_size", inputs); ok {
		in.MaxSize = aws.Int32(int32(n))
	}
	if n, ok := awscommon.InputInt("desired_capacity", inputs); ok {
		in.DesiredCapacity = aws.Int32(int32(n))
	}
	if r := awscommon.InputString("recurrence", inputs); r != "" {
		in.Recurrence = aws.String(r)
	}
	if v := awscommon.InputString("start_time", inputs); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, fmt.Errorf("invalid start_time (expected RFC3339 e.g. 2026-08-01T09:00:00Z): %w", err)
		}
		in.StartTime = aws.Time(t)
	}
	if v := awscommon.InputString("end_time", inputs); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return nil, fmt.Errorf("invalid end_time (expected RFC3339 e.g. 2026-08-31T09:00:00Z): %w", err)
		}
		in.EndTime = aws.Time(t)
	}
	if tz := awscommon.InputString("time_zone", inputs); tz != "" {
		in.TimeZone = aws.String(tz)
	}

	if _, err := client.PutScheduledUpdateGroupAction(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Put scheduled action %s on %s", actionName, groupName),
	}, nil
}
