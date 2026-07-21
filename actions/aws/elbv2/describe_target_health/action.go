// Package aws_elbv2_describe_target_health reports target health for an ELBv2 target group.
package aws_elbv2_describe_target_health

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Describe Target Health"
	Description  = "Report the health of targets in an Elastic Load Balancing target group."
	Website      = "https://www.flomation.co"
	Icon         = "diagram-project+gauge"
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
	{Name: "target_group_arn", Type: core.ConnectionTypeString, Label: "Target Group ARN", Placeholder: "arn:aws:elasticloadbalancing:...:targetgroup/...", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "targets", Type: core.ConnectionTypeString, Label: "Targets"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
	{Name: "healthy_count", Type: core.ConnectionTypeInteger, Label: "Healthy Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	arn := awscommon.InputString("target_group_arn", inputs)
	if arn == "" {
		return nil, fmt.Errorf("target group arn is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	out, err := client.DescribeTargetHealth(ctx, &elasticloadbalancingv2.DescribeTargetHealthInput{
		TargetGroupArn: aws.String(arn),
	})
	if err != nil {
		return nil, err
	}

	var targets []map[string]interface{}
	healthy := 0
	for _, thd := range out.TargetHealthDescriptions {
		entry := map[string]interface{}{}
		if thd.Target != nil {
			entry["id"] = aws.ToString(thd.Target.Id)
			if thd.Target.Port != nil {
				entry["port"] = aws.ToInt32(thd.Target.Port)
			}
		}
		if thd.TargetHealth != nil {
			state := string(thd.TargetHealth.State)
			entry["state"] = state
			entry["reason"] = string(thd.TargetHealth.Reason)
			entry["description"] = aws.ToString(thd.TargetHealth.Description)
			if state == "healthy" {
				healthy++
			}
		}
		targets = append(targets, entry)
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("%d target(s), %d healthy", len(targets), healthy),
		"targets":       targets,
		"count":         len(targets),
		"healthy_count": healthy,
	}, nil
}
