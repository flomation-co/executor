// Package aws_vpc_create_transit_gateway_multicast_domain creates a transit gateway multicast domain.
package aws_vpc_create_transit_gateway_multicast_domain

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
	Name         = "AWS VPC Create Transit Gateway Multicast Domain"
	Description  = "Create a multicast domain on a transit gateway."
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
	{Name: "transit_gateway_id", Type: core.ConnectionTypeString, Label: "Transit Gateway ID", Placeholder: "tgw-0123456789abcdef0", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "multicast_domain", Type: core.ConnectionTypeObject, Label: "Multicast Domain"},
	{Name: "transit_gateway_multicast_domain_id", Type: core.ConnectionTypeString, Label: "Transit Gateway Multicast Domain ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	tgwID := strings.TrimSpace(awscommon.InputString("transit_gateway_id", inputs))
	if tgwID == "" {
		return nil, fmt.Errorf("transit_gateway_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateTransitGatewayMulticastDomainInput{
		TransitGatewayId: aws.String(tgwID),
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeTransitGatewayMulticastDomain,
			Tags:         tags,
		}}
	}

	out, err := client.CreateTransitGatewayMulticastDomain(ctx, in)
	if err != nil {
		return nil, err
	}

	domain := map[string]interface{}{}
	id := ""
	if d := out.TransitGatewayMulticastDomain; d != nil {
		id = aws.ToString(d.TransitGatewayMulticastDomainId)
		domain = map[string]interface{}{
			"transit_gateway_multicast_domain_id":  id,
			"transit_gateway_multicast_domain_arn": aws.ToString(d.TransitGatewayMulticastDomainArn),
			"transit_gateway_id":                   aws.ToString(d.TransitGatewayId),
			"owner_id":                             aws.ToString(d.OwnerId),
			"state":                                string(d.State),
		}
	}

	return map[string]interface{}{
		"tool_result":                         fmt.Sprintf("Created transit gateway multicast domain %s", id),
		"multicast_domain":                    domain,
		"transit_gateway_multicast_domain_id": id,
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
