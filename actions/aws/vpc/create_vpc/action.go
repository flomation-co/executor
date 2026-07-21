// Package aws_vpc_create_vpc creates a new Amazon VPC with the given CIDR block.
package aws_vpc_create_vpc

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Create VPC"
	Description  = "Create a new Amazon VPC with a CIDR block, tenancy, and optional IPv6."
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
	{Name: "cidr_block", Type: core.ConnectionTypeString, Label: "CIDR Block", Placeholder: "10.0.0.0/16", Required: true},
	{Name: "instance_tenancy", Type: core.ConnectionTypeString, Label: "Instance Tenancy", Options: []core.ConnectionOption{
		{Name: "Default", Value: "default"},
		{Name: "Dedicated", Value: "dedicated"},
	}},
	{Name: "amazon_provided_ipv6_cidr_block", Type: core.ConnectionTypeBoolean, Label: "Request Amazon-provided IPv6 CIDR"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vpc", Type: core.ConnectionTypeObject, Label: "VPC"},
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cidr := strings.TrimSpace(awscommon.InputString("cidr_block", inputs))
	if cidr == "" {
		return nil, fmt.Errorf("cidr_block is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateVpcInput{CidrBlock: aws.String(cidr)}
	if t := strings.TrimSpace(awscommon.InputString("instance_tenancy", inputs)); t != "" {
		in.InstanceTenancy = ec2types.Tenancy(t)
	}
	if c := core.FindConnection("amazon_provided_ipv6_cidr_block", inputs); c != nil {
		if b := c.Boolean(); b != nil && *b {
			in.AmazonProvidedIpv6CidrBlock = aws.Bool(true)
		}
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeVpc,
			Tags:         tags,
		}}
	}

	out, err := client.CreateVpc(ctx, in)
	if err != nil {
		return nil, err
	}

	vpc := map[string]interface{}{}
	id := ""
	if out.Vpc != nil {
		id = aws.ToString(out.Vpc.VpcId)
		vpc = map[string]interface{}{
			"vpc_id":     id,
			"cidr_block": aws.ToString(out.Vpc.CidrBlock),
			"state":      string(out.Vpc.State),
			"is_default": aws.ToBool(out.Vpc.IsDefault),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created VPC %s (%s)", id, cidr),
		"vpc":         vpc,
		"vpc_id":      id,
	}, nil
}

func buildTags(inputs []*core.Connection) []ec2types.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []ec2types.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, ec2types.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
