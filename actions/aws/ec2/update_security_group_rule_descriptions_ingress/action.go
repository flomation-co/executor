// Package aws_ec2_update_security_group_rule_descriptions_ingress updates the
// description of an inbound security group rule.
package aws_ec2_update_security_group_rule_descriptions_ingress

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
	Name         = "AWS EC2 Update Security Group Rule Descriptions (Ingress)"
	Description  = "Update the description of an inbound security group rule."
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
	{Name: "description", Type: core.ConnectionTypeString, Label: "Rule Description", Placeholder: "New description text", Required: true},
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
	description := awscommon.InputString("description", inputs)
	if description == "" {
		return nil, fmt.Errorf("description is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	_, err = client.UpdateSecurityGroupRuleDescriptionsIngress(ctx, &ec2.UpdateSecurityGroupRuleDescriptionsIngressInput{
		GroupId: aws.String(groupID),
		SecurityGroupRuleDescriptions: []types.SecurityGroupRuleDescription{{
			SecurityGroupRuleId: aws.String(ruleID),
			Description:         aws.String(description),
		}},
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":            fmt.Sprintf("Updated ingress rule description for %s on %s", ruleID, groupID),
		"security_group_rule_id": ruleID,
	}, nil
}
