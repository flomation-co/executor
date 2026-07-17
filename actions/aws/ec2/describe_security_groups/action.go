// Package aws_ec2_describe_security_groups lists EC2 security groups.
package aws_ec2_describe_security_groups

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EC2 Describe Security Groups"
	Description  = "List security groups with their VPC, description and inbound/outbound rules."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved"
	Date         = "17/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Required: true, Options: []core.ConnectionOption{
		{Name: "Access Keys", Value: "keys"},
		{Name: "Assume Role (cross-account)", Value: "assume_role"},
	}},
	{Name: "aws_access_key", Type: core.ConnectionTypeSecret, Label: "AWS Access Key", Required: true},
	{Name: "aws_secret_key", Type: core.ConnectionTypeSecret, Label: "AWS Secret Key", Required: true},
	{Name: "aws_region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "eu-west-2", Required: true},
	{Name: "aws_session_token", Type: core.ConnectionTypeSecret, Label: "Session Token (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "assume_role_arn", Type: core.ConnectionTypeString, Label: "Assume Role ARN", Placeholder: "arn:aws:iam::123456789012:role/MyRole", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "external_id", Type: core.ConnectionTypeString, Label: "Assume Role External ID (optional)", Placeholder: "Must match the External ID in the role's trust policy", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"assume_role"}}},
	{Name: "group_ids", Type: core.ConnectionTypeString, Label: "Group IDs", Placeholder: "Comma-separated; blank for all (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "security_groups", Type: core.ConnectionTypeObject, Label: "Security Groups"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeSecurityGroupsInput{}
	if ids := awscommon.InputStrings("group_ids", inputs); len(ids) > 0 {
		in.GroupIds = ids
	}

	var groups []map[string]interface{}
	paginator := ec2.NewDescribeSecurityGroupsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range page.SecurityGroups {
			groups = append(groups, map[string]interface{}{
				"group_id":      aws.ToString(g.GroupId),
				"group_name":    aws.ToString(g.GroupName),
				"description":   aws.ToString(g.Description),
				"vpc_id":        aws.ToString(g.VpcId),
				"ingress_rules": rules(g.IpPermissions),
				"egress_rules":  rules(g.IpPermissionsEgress),
			})
		}
	}

	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Found %d security group(s)", len(groups)),
		"security_groups": groups,
		"count":           len(groups),
	}, nil
}

func rules(perms []types.IpPermission) []map[string]interface{} {
	var out []map[string]interface{}
	for _, p := range perms {
		var cidrs []string
		for _, r := range p.IpRanges {
			cidrs = append(cidrs, aws.ToString(r.CidrIp))
		}
		out = append(out, map[string]interface{}{
			"protocol":  aws.ToString(p.IpProtocol),
			"from_port": aws.ToInt32(p.FromPort),
			"to_port":   aws.ToInt32(p.ToPort),
			"cidrs":     cidrs,
		})
	}
	return out
}
