// Package aws_autoscaling_enter_standby moves instances into Standby.
package aws_autoscaling_enter_standby

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Auto Scaling Enter Standby"
	Description  = "Move Auto Scaling group instances into Standby state."
	Website      = "https://www.flomation.co"
	Icon         = "gauge+pen"
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
	{Name: "instance_ids", Type: core.ConnectionTypeString, Label: "Instance IDs (comma-separated, optional)", Placeholder: "i-0abc123,i-0def456"},
	{Name: "should_decrement_desired_capacity", Type: core.ConnectionTypeBoolean, Label: "Decrement Desired Capacity", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "activities", Type: core.ConnectionTypeString, Label: "Activities (JSON)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	groupName := awscommon.InputString("auto_scaling_group_name", inputs)
	if groupName == "" {
		return nil, fmt.Errorf("auto scaling group name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := autoscaling.NewFromConfig(cfg)

	in := &autoscaling.EnterStandbyInput{
		AutoScalingGroupName:           aws.String(groupName),
		ShouldDecrementDesiredCapacity: aws.Bool(awscommon.InputBool("should_decrement_desired_capacity", inputs)),
	}
	if ids := awscommon.InputStrings("instance_ids", inputs); len(ids) > 0 {
		in.InstanceIds = ids
	}

	out, err := client.EnterStandby(ctx, in)
	if err != nil {
		return nil, err
	}

	activities := make([]map[string]interface{}, 0, len(out.Activities))
	for _, a := range out.Activities {
		activities = append(activities, map[string]interface{}{
			"activity_id": aws.ToString(a.ActivityId),
			"description": aws.ToString(a.Description),
			"status_code": string(a.StatusCode),
		})
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Entered standby: %d activities on %s", len(activities), groupName),
		"activities":  activities,
	}, nil
}
