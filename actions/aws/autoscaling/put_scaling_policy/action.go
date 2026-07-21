// Package aws_autoscaling_put_scaling_policy creates or updates a scaling policy.
package aws_autoscaling_put_scaling_policy

import (
	"context"
	"encoding/json"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	astypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Auto Scaling Put Scaling Policy"
	Description  = "Create or update a scaling policy on an Auto Scaling group (target-tracking config is JSON)."
	Website      = "https://www.flomation.co"
	Icon         = "chart-line+plus"
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
	{Name: "policy_name", Type: core.ConnectionTypeString, Label: "Policy Name", Placeholder: "scale-out", Required: true},
	{Name: "policy_type", Type: core.ConnectionTypeString, Label: "Policy Type", Options: []core.ConnectionOption{
		{Name: "Simple Scaling", Value: "SimpleScaling"},
		{Name: "Step Scaling", Value: "StepScaling"},
		{Name: "Target Tracking Scaling", Value: "TargetTrackingScaling"},
		{Name: "Predictive Scaling", Value: "PredictiveScaling"},
	}},
	{Name: "adjustment_type", Type: core.ConnectionTypeString, Label: "Adjustment Type", Options: []core.ConnectionOption{
		{Name: "Change In Capacity", Value: "ChangeInCapacity"},
		{Name: "Exact Capacity", Value: "ExactCapacity"},
		{Name: "Percent Change In Capacity", Value: "PercentChangeInCapacity"},
	}},
	{Name: "scaling_adjustment", Type: core.ConnectionTypeInteger, Label: "Scaling Adjustment (Simple Scaling)", Placeholder: "1"},
	{Name: "cooldown", Type: core.ConnectionTypeInteger, Label: "Cooldown (seconds)", Placeholder: "300"},
	{Name: "target_tracking_configuration", Type: core.ConnectionTypeString, Label: "Target Tracking Configuration (JSON)", Placeholder: `{"TargetValue":50,"PredefinedMetricSpecification":{"PredefinedMetricType":"ASGAverageCPUUtilization"}}`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "policy_arn", Type: core.ConnectionTypeString, Label: "Policy ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	groupName := awscommon.InputString("auto_scaling_group_name", inputs)
	if groupName == "" {
		return nil, fmt.Errorf("auto scaling group name is required")
	}
	policyName := awscommon.InputString("policy_name", inputs)
	if policyName == "" {
		return nil, fmt.Errorf("policy name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := autoscaling.NewFromConfig(cfg)

	in := &autoscaling.PutScalingPolicyInput{
		AutoScalingGroupName: aws.String(groupName),
		PolicyName:           aws.String(policyName),
	}
	if v := awscommon.InputString("policy_type", inputs); v != "" {
		in.PolicyType = aws.String(v)
	}
	if v := awscommon.InputString("adjustment_type", inputs); v != "" {
		in.AdjustmentType = aws.String(v)
	}
	if n, ok := awscommon.InputInt("scaling_adjustment", inputs); ok {
		in.ScalingAdjustment = aws.Int32(int32(n))
	}
	if n, ok := awscommon.InputInt("cooldown", inputs); ok {
		in.Cooldown = aws.Int32(int32(n))
	}
	if raw := awscommon.InputString("target_tracking_configuration", inputs); raw != "" {
		var ttc astypes.TargetTrackingConfiguration
		if err := json.Unmarshal([]byte(raw), &ttc); err != nil {
			return nil, fmt.Errorf("invalid target tracking configuration JSON: %w", err)
		}
		in.TargetTrackingConfiguration = &ttc
	}

	out, err := client.PutScalingPolicy(ctx, in)
	if err != nil {
		return nil, err
	}

	policyARN := aws.ToString(out.PolicyARN)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Put scaling policy %s on %s", policyName, groupName),
		"policy_arn":  policyARN,
	}, nil
}
