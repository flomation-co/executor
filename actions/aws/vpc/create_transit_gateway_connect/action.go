// Package aws_vpc_create_transit_gateway_connect creates a transit gateway Connect attachment.
package aws_vpc_create_transit_gateway_connect

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
	Name         = "AWS VPC Create Transit Gateway Connect"
	Description  = "Create a Connect attachment over an existing VPC or Direct Connect attachment."
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
	{Name: "transport_transit_gateway_attachment_id", Type: core.ConnectionTypeString, Label: "Transport Attachment ID", Placeholder: "tgw-attach-0123456789abcdef0", Required: true},
	{Name: "protocol", Type: core.ConnectionTypeString, Label: "Protocol", Required: true, Options: []core.ConnectionOption{
		{Name: "GRE", Value: "gre"},
	}},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "connect", Type: core.ConnectionTypeObject, Label: "Connect Attachment"},
	{Name: "transit_gateway_attachment_id", Type: core.ConnectionTypeString, Label: "Transit Gateway Attachment ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	transportID := strings.TrimSpace(awscommon.InputString("transport_transit_gateway_attachment_id", inputs))
	if transportID == "" {
		return nil, fmt.Errorf("transport_transit_gateway_attachment_id is required")
	}
	protocol := strings.TrimSpace(awscommon.InputString("protocol", inputs))
	if protocol == "" {
		return nil, fmt.Errorf("protocol is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateTransitGatewayConnectInput{
		TransportTransitGatewayAttachmentId: aws.String(transportID),
		Options: &ec2types.CreateTransitGatewayConnectRequestOptions{
			Protocol: ec2types.ProtocolValue(protocol),
		},
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeTransitGatewayAttachment,
			Tags:         tags,
		}}
	}

	out, err := client.CreateTransitGatewayConnect(ctx, in)
	if err != nil {
		return nil, err
	}

	connect := map[string]interface{}{}
	id := ""
	if c := out.TransitGatewayConnect; c != nil {
		id = aws.ToString(c.TransitGatewayAttachmentId)
		connect = map[string]interface{}{
			"transit_gateway_attachment_id":           id,
			"transit_gateway_id":                      aws.ToString(c.TransitGatewayId),
			"transport_transit_gateway_attachment_id": aws.ToString(c.TransportTransitGatewayAttachmentId),
			"state": string(c.State),
		}
		if c.Options != nil {
			connect["protocol"] = string(c.Options.Protocol)
		}
	}

	return map[string]interface{}{
		"tool_result":                   fmt.Sprintf("Created transit gateway Connect attachment %s", id),
		"connect":                       connect,
		"transit_gateway_attachment_id": id,
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
