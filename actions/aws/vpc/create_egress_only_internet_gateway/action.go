// Package aws_vpc_create_egress_only_internet_gateway creates an egress-only
// internet gateway for IPv6 outbound traffic.
package aws_vpc_create_egress_only_internet_gateway

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
	Name         = "AWS VPC Create Egress-Only Internet Gateway"
	Description  = "Create an egress-only internet gateway for outbound IPv6 traffic in a VPC."
	Website      = "https://www.flomation.co"
	Icon         = "globe+plus"
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
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID", Placeholder: "vpc-0abc", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "egress_only_internet_gateway", Type: core.ConnectionTypeObject, Label: "Egress-Only Internet Gateway"},
	{Name: "egress_only_internet_gateway_id", Type: core.ConnectionTypeString, Label: "Egress-Only Internet Gateway ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	vpcID := strings.TrimSpace(awscommon.InputString("vpc_id", inputs))
	if vpcID == "" {
		return nil, fmt.Errorf("vpc_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateEgressOnlyInternetGatewayInput{VpcId: aws.String(vpcID)}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeEgressOnlyInternetGateway,
			Tags:         tags,
		}}
	}

	out, err := client.CreateEgressOnlyInternetGateway(ctx, in)
	if err != nil {
		return nil, err
	}

	gw := map[string]interface{}{}
	id := ""
	if out.EgressOnlyInternetGateway != nil {
		g := out.EgressOnlyInternetGateway
		id = aws.ToString(g.EgressOnlyInternetGatewayId)
		state := ""
		vpc := ""
		if len(g.Attachments) > 0 {
			state = string(g.Attachments[0].State)
			vpc = aws.ToString(g.Attachments[0].VpcId)
		}
		gw = map[string]interface{}{
			"egress_only_internet_gateway_id": id,
			"state":                           state,
			"vpc_id":                          vpc,
		}
	}

	return map[string]interface{}{
		"tool_result":                     fmt.Sprintf("Created egress-only internet gateway %s in %s", id, vpcID),
		"egress_only_internet_gateway":    gw,
		"egress_only_internet_gateway_id": id,
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
