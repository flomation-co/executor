// Package aws_elbv2_create_target_group creates an ELBv2 target group.
package aws_elbv2_create_target_group

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
	elbv2types "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Create Target Group"
	Description  = "Create an Elastic Load Balancing target group."
	Website      = "https://www.flomation.co"
	Icon         = "diagram-project+plus"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Target Group Name", Placeholder: "my-targets", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Options: []core.ConnectionOption{
		{Name: "HTTP", Value: "HTTP"},
		{Name: "HTTPS", Value: "HTTPS"},
		{Name: "TCP", Value: "TCP"},
		{Name: "TLS", Value: "TLS"},
		{Name: "UDP", Value: "UDP"},
		{Name: "TCP_UDP", Value: "TCP_UDP"},
		{Name: "GENEVE", Value: "GENEVE"},
	}},
	{Name: "port", Type: core.ConnectionTypeInteger, Label: "Port", Placeholder: "80"},
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID", Placeholder: "vpc-0abc123"},
	{Name: "target_type", Type: core.ConnectionTypeString, Label: "Target Type", Options: []core.ConnectionOption{
		{Name: "Instance", Value: "instance"},
		{Name: "IP", Value: "ip"},
		{Name: "Lambda", Value: "lambda"},
		{Name: "ALB", Value: "alb"},
	}},
	{Name: "health_check_protocol", Type: core.ConnectionTypeString, Label: "Health Check Protocol", Placeholder: "HTTP"},
	{Name: "health_check_path", Type: core.ConnectionTypeString, Label: "Health Check Path", Placeholder: "/health"},
	{Name: "health_check_port", Type: core.ConnectionTypeString, Label: "Health Check Port", Placeholder: "traffic-port"},
	{Name: "health_check_interval_seconds", Type: core.ConnectionTypeInteger, Label: "Health Check Interval (seconds)", Placeholder: "30"},
	{Name: "healthy_threshold_count", Type: core.ConnectionTypeInteger, Label: "Healthy Threshold Count", Placeholder: "5"},
	{Name: "unhealthy_threshold_count", Type: core.ConnectionTypeInteger, Label: "Unhealthy Threshold Count", Placeholder: "2"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "target_group_arn", Type: core.ConnectionTypeString, Label: "Target Group ARN"},
	{Name: "target_group_name", Type: core.ConnectionTypeString, Label: "Target Group Name"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("target group name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	in := &elasticloadbalancingv2.CreateTargetGroupInput{Name: aws.String(name)}
	if v := awscommon.InputString("protocol", inputs); v != "" {
		in.Protocol = elbv2types.ProtocolEnum(v)
	}
	if v, ok := awscommon.InputInt("port", inputs); ok {
		in.Port = aws.Int32(int32(v))
	}
	if v := awscommon.InputString("vpc_id", inputs); v != "" {
		in.VpcId = aws.String(v)
	}
	if v := awscommon.InputString("target_type", inputs); v != "" {
		in.TargetType = elbv2types.TargetTypeEnum(v)
	}
	if v := awscommon.InputString("health_check_protocol", inputs); v != "" {
		in.HealthCheckProtocol = elbv2types.ProtocolEnum(v)
	}
	if v := awscommon.InputString("health_check_path", inputs); v != "" {
		in.HealthCheckPath = aws.String(v)
	}
	if v := awscommon.InputString("health_check_port", inputs); v != "" {
		in.HealthCheckPort = aws.String(v)
	}
	if v, ok := awscommon.InputInt("health_check_interval_seconds", inputs); ok {
		in.HealthCheckIntervalSeconds = aws.Int32(int32(v))
	}
	if v, ok := awscommon.InputInt("healthy_threshold_count", inputs); ok {
		in.HealthyThresholdCount = aws.Int32(int32(v))
	}
	if v, ok := awscommon.InputInt("unhealthy_threshold_count", inputs); ok {
		in.UnhealthyThresholdCount = aws.Int32(int32(v))
	}

	out, err := client.CreateTargetGroup(ctx, in)
	if err != nil {
		return nil, err
	}
	if len(out.TargetGroups) == 0 {
		return nil, fmt.Errorf("no target group returned")
	}
	tg := out.TargetGroups[0]
	arn := aws.ToString(tg.TargetGroupArn)
	tgName := aws.ToString(tg.TargetGroupName)
	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Created target group %s (%s)", tgName, arn),
		"target_group_arn":  arn,
		"target_group_name": tgName,
	}, nil
}
