// Package aws_elbv2_set_security_groups sets the security groups for an Elastic
// Load Balancing v2 load balancer.
package aws_elbv2_set_security_groups

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	elbv2 "github.com/aws/aws-sdk-go-v2/service/elasticloadbalancingv2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Set Load Balancer Security Groups"
	Description  = "Set the security groups associated with a load balancer."
	Website      = "https://www.flomation.co"
	Icon         = "arrows-split-up-and-left+shield-halved"
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
	{Name: "load_balancer_arn", Type: core.ConnectionTypeString, Label: "Load Balancer ARN", Placeholder: "arn:aws:elasticloadbalancing:...", Required: true},
	{Name: "security_groups", Type: core.ConnectionTypeString, Label: "Security Group IDs", Placeholder: "Comma-separated security group IDs", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "security_group_ids", Type: core.ConnectionTypeObject, Label: "Security Group IDs"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	arn := strings.TrimSpace(awscommon.InputString("load_balancer_arn", inputs))
	if arn == "" {
		return nil, fmt.Errorf("load balancer arn is required")
	}
	sgs := awscommon.InputStrings("security_groups", inputs)
	if len(sgs) == 0 {
		return nil, fmt.Errorf("at least one security group is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := elbv2.NewFromConfig(cfg)

	out, err := client.SetSecurityGroups(ctx, &elbv2.SetSecurityGroupsInput{
		LoadBalancerArn: aws.String(arn),
		SecurityGroups:  sgs,
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Set %d security group(s) on %s", len(out.SecurityGroupIds), arn),
		"security_group_ids": out.SecurityGroupIds,
	}, nil
}
