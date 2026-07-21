// Package aws_elbv2_describe_target_groups lists ELBv2 target groups.
package aws_elbv2_describe_target_groups

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
	Name         = "AWS Describe Target Groups"
	Description  = "List Elastic Load Balancing target groups."
	Website      = "https://www.flomation.co"
	Icon         = "diagram-project+magnifying-glass"
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
	{Name: "target_group_arns", Type: core.ConnectionTypeString, Label: "Target Group ARNs (comma-separated)", Placeholder: "arn:...,arn:..."},
	{Name: "names", Type: core.ConnectionTypeString, Label: "Names (comma-separated)", Placeholder: "my-targets"},
	{Name: "load_balancer_arn", Type: core.ConnectionTypeString, Label: "Load Balancer ARN", Placeholder: "arn:aws:elasticloadbalancing:..."},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "target_groups", Type: core.ConnectionTypeString, Label: "Target Groups"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elasticloadbalancingv2.NewFromConfig(cfg)

	in := &elasticloadbalancingv2.DescribeTargetGroupsInput{}
	if arns := awscommon.InputStrings("target_group_arns", inputs); len(arns) > 0 {
		in.TargetGroupArns = arns
	}
	if names := awscommon.InputStrings("names", inputs); len(names) > 0 {
		in.Names = names
	}
	if lb := awscommon.InputString("load_balancer_arn", inputs); lb != "" {
		in.LoadBalancerArn = aws.String(lb)
	}

	var groups []map[string]interface{}
	paginator := elasticloadbalancingv2.NewDescribeTargetGroupsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, tg := range page.TargetGroups {
			entry := map[string]interface{}{
				"arn":         aws.ToString(tg.TargetGroupArn),
				"name":        aws.ToString(tg.TargetGroupName),
				"protocol":    string(tg.Protocol),
				"vpc_id":      aws.ToString(tg.VpcId),
				"target_type": string(tg.TargetType),
			}
			if tg.Port != nil {
				entry["port"] = aws.ToInt32(tg.Port)
			}
			groups = append(groups, entry)
		}
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Found %d target group(s)", len(groups)),
		"target_groups": groups,
		"count":         len(groups),
	}, nil
}
