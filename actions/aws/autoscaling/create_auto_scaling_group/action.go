// Package aws_autoscaling_create_auto_scaling_group creates an EC2 Auto Scaling group.
package aws_autoscaling_create_auto_scaling_group

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/autoscaling"
	astypes "github.com/aws/aws-sdk-go-v2/service/autoscaling/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Create Auto Scaling Group"
	Description  = "Create an EC2 Auto Scaling group from a launch template."
	Website      = "https://www.flomation.co"
	Icon         = "arrows-up-down+plus"
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
	{Name: "min_size", Type: core.ConnectionTypeInteger, Label: "Minimum Size", Required: true},
	{Name: "max_size", Type: core.ConnectionTypeInteger, Label: "Maximum Size", Required: true},
	{Name: "desired_capacity", Type: core.ConnectionTypeInteger, Label: "Desired Capacity (optional)"},
	{Name: "launch_template_id", Type: core.ConnectionTypeString, Label: "Launch Template ID", Placeholder: "lt-0abc123"},
	{Name: "launch_template_name", Type: core.ConnectionTypeString, Label: "Launch Template Name"},
	{Name: "launch_template_version", Type: core.ConnectionTypeString, Label: "Launch Template Version", Placeholder: "$Default"},
	{Name: "vpc_zone_identifier", Type: core.ConnectionTypeString, Label: "Subnet IDs (comma-separated)", Placeholder: "subnet-abc,subnet-def"},
	{Name: "availability_zones", Type: core.ConnectionTypeString, Label: "Availability Zones (comma-separated, optional)"},
	{Name: "target_group_arns", Type: core.ConnectionTypeString, Label: "Target Group ARNs (comma-separated, optional)"},
	{Name: "health_check_type", Type: core.ConnectionTypeString, Label: "Health Check Type", Options: []core.ConnectionOption{
		{Name: "EC2", Value: "EC2"},
		{Name: "ELB", Value: "ELB"},
	}},
	{Name: "health_check_grace_period", Type: core.ConnectionTypeInteger, Label: "Health Check Grace Period (seconds)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "auto_scaling_group_name", Type: core.ConnectionTypeString, Label: "Auto Scaling Group Name"},
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("auto_scaling_group_name", inputs))
	if name == "" {
		return nil, fmt.Errorf("auto scaling group name is required")
	}
	minSize, ok := awscommon.InputInt("min_size", inputs)
	if !ok {
		return nil, fmt.Errorf("minimum size is required")
	}
	maxSize, ok := awscommon.InputInt("max_size", inputs)
	if !ok {
		return nil, fmt.Errorf("maximum size is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := autoscaling.NewFromConfig(cfg)

	in := &autoscaling.CreateAutoScalingGroupInput{
		AutoScalingGroupName: aws.String(name),
		MinSize:              aws.Int32(int32(minSize)),
		MaxSize:              aws.Int32(int32(maxSize)),
	}

	if dc, ok := awscommon.InputInt("desired_capacity", inputs); ok {
		in.DesiredCapacity = aws.Int32(int32(dc))
	}

	ltID := strings.TrimSpace(awscommon.InputString("launch_template_id", inputs))
	ltName := strings.TrimSpace(awscommon.InputString("launch_template_name", inputs))
	if ltID != "" || ltName != "" {
		lt := &astypes.LaunchTemplateSpecification{}
		if ltID != "" {
			lt.LaunchTemplateId = aws.String(ltID)
		}
		if ltName != "" {
			lt.LaunchTemplateName = aws.String(ltName)
		}
		if v := strings.TrimSpace(awscommon.InputString("launch_template_version", inputs)); v != "" {
			lt.Version = aws.String(v)
		} else {
			lt.Version = aws.String("$Default")
		}
		in.LaunchTemplate = lt
	}

	if v := strings.TrimSpace(awscommon.InputString("vpc_zone_identifier", inputs)); v != "" {
		in.VPCZoneIdentifier = aws.String(strings.Join(splitList(v), ","))
	}
	if azs := splitList(awscommon.InputString("availability_zones", inputs)); len(azs) > 0 {
		in.AvailabilityZones = azs
	}
	if arns := splitList(awscommon.InputString("target_group_arns", inputs)); len(arns) > 0 {
		in.TargetGroupARNs = arns
	}
	if hct := strings.TrimSpace(awscommon.InputString("health_check_type", inputs)); hct != "" {
		in.HealthCheckType = aws.String(hct)
	}
	if gp, ok := awscommon.InputInt("health_check_grace_period", inputs); ok {
		in.HealthCheckGracePeriod = aws.Int32(int32(gp))
	}

	if conn := core.FindConnection("tags", inputs); conn != nil {
		for _, kv := range conn.KeyValuePairs() {
			k := strings.TrimSpace(kv.Key)
			if k == "" {
				continue
			}
			in.Tags = append(in.Tags, astypes.Tag{
				Key:               aws.String(k),
				Value:             aws.String(kv.Value),
				PropagateAtLaunch: aws.Bool(true),
			})
		}
	}

	if _, err := client.CreateAutoScalingGroup(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":             fmt.Sprintf("Created Auto Scaling group %s (min %d, max %d)", name, minSize, maxSize),
		"auto_scaling_group_name": name,
	}, nil
}
