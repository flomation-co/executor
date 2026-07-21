// Package aws_vpc_create_subnet creates a subnet within a VPC.
package aws_vpc_create_subnet

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
	Name         = "AWS VPC Create Subnet"
	Description  = "Create a subnet within a VPC with a CIDR block and optional AZ."
	Website      = "https://www.flomation.co"
	Icon         = "object-group+plus"
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
	{Name: "cidr_block", Type: core.ConnectionTypeString, Label: "CIDR Block", Placeholder: "10.0.1.0/24", Required: true},
	{Name: "availability_zone", Type: core.ConnectionTypeString, Label: "Availability Zone (optional)", Placeholder: "eu-west-2a"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subnet", Type: core.ConnectionTypeObject, Label: "Subnet"},
	{Name: "subnet_id", Type: core.ConnectionTypeString, Label: "Subnet ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	vpcID := strings.TrimSpace(awscommon.InputString("vpc_id", inputs))
	if vpcID == "" {
		return nil, fmt.Errorf("vpc_id is required")
	}
	cidr := strings.TrimSpace(awscommon.InputString("cidr_block", inputs))
	if cidr == "" {
		return nil, fmt.Errorf("cidr_block is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateSubnetInput{
		VpcId:     aws.String(vpcID),
		CidrBlock: aws.String(cidr),
	}
	if az := strings.TrimSpace(awscommon.InputString("availability_zone", inputs)); az != "" {
		in.AvailabilityZone = aws.String(az)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeSubnet,
			Tags:         tags,
		}}
	}

	out, err := client.CreateSubnet(ctx, in)
	if err != nil {
		return nil, err
	}

	subnet := map[string]interface{}{}
	id := ""
	if out.Subnet != nil {
		id = aws.ToString(out.Subnet.SubnetId)
		subnet = map[string]interface{}{
			"subnet_id":         id,
			"vpc_id":            aws.ToString(out.Subnet.VpcId),
			"cidr_block":        aws.ToString(out.Subnet.CidrBlock),
			"availability_zone": aws.ToString(out.Subnet.AvailabilityZone),
			"state":             string(out.Subnet.State),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created subnet %s (%s)", id, cidr),
		"subnet":      subnet,
		"subnet_id":   id,
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
