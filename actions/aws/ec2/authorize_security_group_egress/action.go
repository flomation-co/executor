// Package aws_ec2_authorize_security_group_egress adds an outbound rule to a
// security group.
package aws_ec2_authorize_security_group_egress

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
	Name         = "AWS EC2 Authorize Security Group Egress"
	Description  = "Add an outbound rule to a security group (protocol, port range and CIDR)."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+plus"
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
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Required: true, Options: []core.ConnectionOption{
		{Name: "TCP", Value: "tcp"},
		{Name: "UDP", Value: "udp"},
		{Name: "ICMP", Value: "icmp"},
		{Name: "All", Value: "-1"},
	}},
	{Name: "from_port", Type: core.ConnectionTypeInteger, Label: "From Port", Placeholder: "e.g. 443"},
	{Name: "to_port", Type: core.ConnectionTypeInteger, Label: "To Port", Placeholder: "e.g. 443"},
	{Name: "cidr", Type: core.ConnectionTypeString, Label: "CIDR", Placeholder: "0.0.0.0/0", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Rule Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "group_id", Type: core.ConnectionTypeString, Label: "Group ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	groupID := awscommon.InputString("group_id", inputs)
	if groupID == "" {
		return nil, fmt.Errorf("group id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	_, err = client.AuthorizeSecurityGroupEgress(ctx, &ec2.AuthorizeSecurityGroupEgressInput{
		GroupId:       aws.String(groupID),
		IpPermissions: []types.IpPermission{buildPermission(inputs)},
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Authorized egress on %s", groupID),
		"group_id":    groupID,
	}, nil
}

// buildPermission assembles a single egress rule from the protocol/port/CIDR
// inputs. Ports are omitted for the "all protocols" (-1) case.
func buildPermission(inputs []*core.Connection) types.IpPermission {
	perm := types.IpPermission{
		IpProtocol: aws.String(awscommon.InputString("protocol", inputs)),
		IpRanges: []types.IpRange{{
			CidrIp:      aws.String(awscommon.InputString("cidr", inputs)),
			Description: aws.String(awscommon.InputString("description", inputs)),
		}},
	}
	if p := core.FindConnection("from_port", inputs); p != nil {
		if n := p.Number(); n != nil {
			perm.FromPort = aws.Int32(int32(*n))
		}
	}
	if p := core.FindConnection("to_port", inputs); p != nil {
		if n := p.Number(); n != nil {
			perm.ToPort = aws.Int32(int32(*n))
		}
	}
	return perm
}
