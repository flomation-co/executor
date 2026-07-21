// Package aws_vpc_create_nat_gateway creates a NAT gateway in a subnet.
package aws_vpc_create_nat_gateway

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
	Name         = "AWS VPC Create NAT Gateway"
	Description  = "Create a NAT gateway in a subnet (public with an EIP, or private)."
	Website      = "https://www.flomation.co"
	Icon         = "route+plus"
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
	{Name: "subnet_id", Type: core.ConnectionTypeString, Label: "Subnet ID", Placeholder: "subnet-0abc123", Required: true},
	{Name: "allocation_id", Type: core.ConnectionTypeString, Label: "Elastic IP Allocation ID", Placeholder: "eipalloc-0abc (required for a public NAT gateway)"},
	{Name: "connectivity_type", Type: core.ConnectionTypeString, Label: "Connectivity Type", Options: []core.ConnectionOption{
		{Name: "Public", Value: "public"},
		{Name: "Private", Value: "private"},
	}},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags", Placeholder: "Optional tags to apply to the NAT gateway"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "nat_gateway", Type: core.ConnectionTypeObject, Label: "NAT Gateway"},
	{Name: "nat_gateway_id", Type: core.ConnectionTypeString, Label: "NAT Gateway ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	subnetID := awscommon.InputString("subnet_id", inputs)
	if subnetID == "" {
		return nil, fmt.Errorf("subnet id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateNatGatewayInput{SubnetId: aws.String(subnetID)}
	if v := awscommon.InputString("allocation_id", inputs); v != "" {
		in.AllocationId = aws.String(v)
	}
	if v := awscommon.InputString("connectivity_type", inputs); v != "" {
		in.ConnectivityType = ec2types.ConnectivityType(v)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeNatgateway,
			Tags:         tags,
		}}
	}

	out, err := client.CreateNatGateway(ctx, in)
	if err != nil {
		return nil, err
	}

	var natID string
	var summary map[string]interface{}
	if out.NatGateway != nil {
		natID = aws.ToString(out.NatGateway.NatGatewayId)
		var addresses []map[string]interface{}
		for _, a := range out.NatGateway.NatGatewayAddresses {
			addresses = append(addresses, map[string]interface{}{
				"allocation_id": aws.ToString(a.AllocationId),
				"public_ip":     aws.ToString(a.PublicIp),
				"private_ip":    aws.ToString(a.PrivateIp),
			})
		}
		summary = map[string]interface{}{
			"nat_gateway_id":    natID,
			"vpc_id":            aws.ToString(out.NatGateway.VpcId),
			"subnet_id":         aws.ToString(out.NatGateway.SubnetId),
			"state":             string(out.NatGateway.State),
			"connectivity_type": string(out.NatGateway.ConnectivityType),
			"addresses":         addresses,
		}
	}

	return map[string]interface{}{
		"tool_result":    "Created NAT gateway " + natID,
		"nat_gateway":    summary,
		"nat_gateway_id": natID,
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
