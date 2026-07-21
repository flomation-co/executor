// Package aws_autoscaling_set_desired_capacity sets an Auto Scaling group's desired capacity.
package aws_autoscaling_set_desired_capacity

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Set Desired Capacity"
	Description  = "Set the desired capacity of an EC2 Auto Scaling group."
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
	{Name: "desired_capacity", Type: core.ConnectionTypeInteger, Label: "Desired Capacity", Required: true},
	{Name: "honor_cooldown", Type: core.ConnectionTypeBoolean, Label: "Honour Cooldown"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "auto_scaling_group_name", Type: core.ConnectionTypeString, Label: "Auto Scaling Group Name"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("auto_scaling_group_name", inputs))
	if name == "" {
		return nil, fmt.Errorf("auto scaling group name is required")
	}
	dc, ok := awscommon.InputInt("desired_capacity", inputs)
	if !ok {
		return nil, fmt.Errorf("desired capacity is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := autoscaling.NewFromConfig(cfg)

	in := &autoscaling.SetDesiredCapacityInput{
		AutoScalingGroupName: aws.String(name),
		DesiredCapacity:      aws.Int32(int32(dc)),
	}
	if awscommon.InputBool("honor_cooldown", inputs) {
		in.HonorCooldown = aws.Bool(true)
	}

	if _, err := client.SetDesiredCapacity(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":             fmt.Sprintf("Set desired capacity of %s to %d", name, dc),
		"auto_scaling_group_name": name,
	}, nil
}
