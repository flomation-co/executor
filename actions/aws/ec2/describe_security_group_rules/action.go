// Package aws_ec2_describe_security_group_rules lists individual security group
// rules (ingress and egress) by ID or group.
package aws_ec2_describe_security_group_rules

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
	Name         = "AWS EC2 Describe Security Group Rules"
	Description  = "List individual security group rules (ingress and egress) by rule ID or group."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+magnifying-glass"
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
	{Name: "security_group_rule_ids", Type: core.ConnectionTypeString, Label: "Security Group Rule IDs", Placeholder: "Comma-separated; blank for all (optional)"},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Filter by Group ID", Placeholder: "sg-0abc123 (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rules", Type: core.ConnectionTypeObject, Label: "Security Group Rules"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeSecurityGroupRulesInput{}
	if ids := awscommon.InputStrings("security_group_rule_ids", inputs); len(ids) > 0 {
		in.SecurityGroupRuleIds = ids
	}
	if filters := awscommon.BuildEC2Filters(inputs, []awscommon.FilterSpec{
		{Input: "group_id", Filter: "group-id"},
	}); len(filters) > 0 {
		in.Filters = filters
	}

	var rules []map[string]interface{}
	paginator := ec2.NewDescribeSecurityGroupRulesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, r := range page.SecurityGroupRules {
			rules = append(rules, rule(r))
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d security group rule(s)", len(rules)),
		"rules":       rules,
		"count":       len(rules),
	}, nil
}

func rule(r types.SecurityGroupRule) map[string]interface{} {
	return map[string]interface{}{
		"security_group_rule_id": aws.ToString(r.SecurityGroupRuleId),
		"group_id":               aws.ToString(r.GroupId),
		"is_egress":              aws.ToBool(r.IsEgress),
		"ip_protocol":            aws.ToString(r.IpProtocol),
		"from_port":              aws.ToInt32(r.FromPort),
		"to_port":                aws.ToInt32(r.ToPort),
		"cidr_ipv4":              aws.ToString(r.CidrIpv4),
		"description":            aws.ToString(r.Description),
	}
}
