// Package aws_ec2_modify_security_group_rules updates an existing security group
// rule in place.
package aws_ec2_modify_security_group_rules

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
	Name         = "AWS EC2 Modify Security Group Rules"
	Description  = "Update an existing security group rule (protocol, port range, CIDR and description)."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+pen"
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
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group ID", Placeholder: "sg-0abc123", Required: true},
	{Name: "security_group_rule_id", Type: core.ConnectionTypeString, Label: "Security Group Rule ID", Placeholder: "sgr-0abc123", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Options: []core.ConnectionOption{
		{Name: "TCP", Value: "tcp"},
		{Name: "UDP", Value: "udp"},
		{Name: "ICMP", Value: "icmp"},
		{Name: "All", Value: "-1"},
	}},
	{Name: "from_port", Type: core.ConnectionTypeInteger, Label: "From Port", Placeholder: "e.g. 443"},
	{Name: "to_port", Type: core.ConnectionTypeInteger, Label: "To Port", Placeholder: "e.g. 443"},
	{Name: "cidr_ipv4", Type: core.ConnectionTypeString, Label: "CIDR (IPv4)", Placeholder: "0.0.0.0/0 (optional)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Rule Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "security_group_rule_id", Type: core.ConnectionTypeString, Label: "Security Group Rule ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	groupID := awscommon.InputString("group_id", inputs)
	if groupID == "" {
		return nil, fmt.Errorf("group id is required")
	}
	ruleID := awscommon.InputString("security_group_rule_id", inputs)
	if ruleID == "" {
		return nil, fmt.Errorf("security group rule id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	req := types.SecurityGroupRuleRequest{}
	if p := awscommon.InputString("protocol", inputs); p != "" {
		req.IpProtocol = aws.String(p)
	}
	if c := awscommon.InputString("cidr_ipv4", inputs); c != "" {
		req.CidrIpv4 = aws.String(c)
	}
	if d := awscommon.InputString("description", inputs); d != "" {
		req.Description = aws.String(d)
	}
	if p := core.FindConnection("from_port", inputs); p != nil {
		if n := p.Number(); n != nil {
			req.FromPort = aws.Int32(int32(*n))
		}
	}
	if p := core.FindConnection("to_port", inputs); p != nil {
		if n := p.Number(); n != nil {
			req.ToPort = aws.Int32(int32(*n))
		}
	}

	_, err = client.ModifySecurityGroupRules(ctx, &ec2.ModifySecurityGroupRulesInput{
		GroupId: aws.String(groupID),
		SecurityGroupRules: []types.SecurityGroupRuleUpdate{{
			SecurityGroupRuleId: aws.String(ruleID),
			SecurityGroupRule:   &req,
		}},
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":            fmt.Sprintf("Modified rule %s on %s", ruleID, groupID),
		"security_group_rule_id": ruleID,
	}, nil
}
