// Package aws_vpc_associate_vpc_cidr_block associates a secondary CIDR block with a VPC.
package aws_vpc_associate_vpc_cidr_block

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Associate CIDR Block"
	Description  = "Associate a secondary IPv4 or IPv6 CIDR block with a VPC."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+plus"
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
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID", Placeholder: "vpc-0abc...", Required: true},
	{Name: "cidr_block", Type: core.ConnectionTypeString, Label: "Secondary IPv4 CIDR Block (optional)", Placeholder: "10.1.0.0/16"},
	{Name: "amazon_provided_ipv6_cidr_block", Type: core.ConnectionTypeBoolean, Label: "Request Amazon-provided IPv6 CIDR"},
	{Name: "ipv6_cidr_block", Type: core.ConnectionTypeString, Label: "IPv6 CIDR Block (optional)", Placeholder: "2600:1f16:...::/56"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "cidr_association", Type: core.ConnectionTypeObject, Label: "CIDR Association"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	vpcID := strings.TrimSpace(awscommon.InputString("vpc_id", inputs))
	if vpcID == "" {
		return nil, fmt.Errorf("vpc_id is required")
	}
	cidr := strings.TrimSpace(awscommon.InputString("cidr_block", inputs))
	ipv6Cidr := strings.TrimSpace(awscommon.InputString("ipv6_cidr_block", inputs))
	amazonIPv6 := false
	if c := core.FindConnection("amazon_provided_ipv6_cidr_block", inputs); c != nil {
		if b := c.Boolean(); b != nil {
			amazonIPv6 = *b
		}
	}
	if cidr == "" && ipv6Cidr == "" && !amazonIPv6 {
		return nil, fmt.Errorf("provide an IPv4 CIDR block, an IPv6 CIDR block, or request an Amazon-provided IPv6 CIDR")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.AssociateVpcCidrBlockInput{VpcId: aws.String(vpcID)}
	if cidr != "" {
		in.CidrBlock = aws.String(cidr)
	}
	if ipv6Cidr != "" {
		in.Ipv6CidrBlock = aws.String(ipv6Cidr)
	}
	if amazonIPv6 {
		in.AmazonProvidedIpv6CidrBlock = aws.Bool(true)
	}

	out, err := client.AssociateVpcCidrBlock(ctx, in)
	if err != nil {
		return nil, err
	}

	assoc := map[string]interface{}{}
	summary := "unknown CIDR"
	if out.CidrBlockAssociation != nil {
		a := out.CidrBlockAssociation
		assoc = map[string]interface{}{
			"association_id": aws.ToString(a.AssociationId),
			"cidr_block":     aws.ToString(a.CidrBlock),
		}
		if a.CidrBlockState != nil {
			assoc["state"] = string(a.CidrBlockState.State)
		}
		summary = aws.ToString(a.CidrBlock)
	} else if out.Ipv6CidrBlockAssociation != nil {
		a := out.Ipv6CidrBlockAssociation
		assoc = map[string]interface{}{
			"association_id":  aws.ToString(a.AssociationId),
			"ipv6_cidr_block": aws.ToString(a.Ipv6CidrBlock),
		}
		if a.Ipv6CidrBlockState != nil {
			assoc["state"] = string(a.Ipv6CidrBlockState.State)
		}
		summary = aws.ToString(a.Ipv6CidrBlock)
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Associated CIDR %s with VPC %s", summary, vpcID),
		"cidr_association": assoc,
	}, nil
}
