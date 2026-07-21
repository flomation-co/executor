// Package aws_autoscaling_detach_instances detaches EC2 instances from an Auto Scaling group.
package aws_autoscaling_detach_instances

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
	Name         = "AWS Detach Instances"
	Description  = "Detach EC2 instances from an Auto Scaling group."
	Website      = "https://www.flomation.co"
	Icon         = "server+arrow-down"
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
	{Name: "instance_ids", Type: core.ConnectionTypeString, Label: "Instance IDs (comma-separated)", Placeholder: "i-abc123,i-def456", Required: true},
	{Name: "should_decrement_desired_capacity", Type: core.ConnectionTypeBoolean, Label: "Decrement Desired Capacity", Required: true},
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
	ids := awscommon.InputStrings("instance_ids", inputs)
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one instance id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := autoscaling.NewFromConfig(cfg)

	in := &autoscaling.DetachInstancesInput{
		AutoScalingGroupName:           aws.String(name),
		InstanceIds:                    ids,
		ShouldDecrementDesiredCapacity: aws.Bool(awscommon.InputBool("should_decrement_desired_capacity", inputs)),
	}

	if _, err := client.DetachInstances(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":             fmt.Sprintf("Detached %d instance(s) from %s", len(ids), name),
		"auto_scaling_group_name": name,
	}, nil
}
