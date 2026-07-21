// Package aws_autoscaling_set_instance_health sets the health status of an Auto Scaling instance.
package aws_autoscaling_set_instance_health

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
	Name         = "AWS Set Instance Health"
	Description  = "Set the health status of an instance in an Auto Scaling group."
	Website      = "https://www.flomation.co"
	Icon         = "server+gauge"
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
	{Name: "instance_id", Type: core.ConnectionTypeString, Label: "Instance ID", Placeholder: "i-abc123", Required: true},
	{Name: "health_status", Type: core.ConnectionTypeString, Label: "Health Status", Required: true, Options: []core.ConnectionOption{
		{Name: "Healthy", Value: "Healthy"},
		{Name: "Unhealthy", Value: "Unhealthy"},
	}},
	{Name: "should_respect_grace_period", Type: core.ConnectionTypeBoolean, Label: "Respect Health Check Grace Period"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance_id", Type: core.ConnectionTypeString, Label: "Instance ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	instanceID := strings.TrimSpace(awscommon.InputString("instance_id", inputs))
	if instanceID == "" {
		return nil, fmt.Errorf("instance id is required")
	}
	status := strings.TrimSpace(awscommon.InputString("health_status", inputs))
	if status == "" {
		return nil, fmt.Errorf("health status is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := autoscaling.NewFromConfig(cfg)

	in := &autoscaling.SetInstanceHealthInput{
		InstanceId:   aws.String(instanceID),
		HealthStatus: aws.String(status),
	}
	if awscommon.InputBool("should_respect_grace_period", inputs) {
		in.ShouldRespectGracePeriod = aws.Bool(true)
	}

	if _, err := client.SetInstanceHealth(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Set instance %s health to %s", instanceID, status),
		"instance_id": instanceID,
	}, nil
}
