// Package aws_vpc_create_transit_gateway_peering_attachment requests a transit gateway peering attachment.
package aws_vpc_create_transit_gateway_peering_attachment

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
	Name         = "AWS VPC Create Transit Gateway Peering Attachment"
	Description  = "Request a peering attachment between two transit gateways across accounts or Regions."
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
	{Name: "peer_transit_gateway_id", Type: core.ConnectionTypeString, Label: "Peer Transit Gateway ID", Placeholder: "tgw-0123456789abcdef0", Required: true},
	{Name: "peer_account_id", Type: core.ConnectionTypeString, Label: "Peer Account ID", Placeholder: "123456789012", Required: true},
	{Name: "peer_region", Type: core.ConnectionTypeString, Label: "Peer Region", Placeholder: "us-east-1", Required: true},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attachment", Type: core.ConnectionTypeObject, Label: "Attachment"},
	{Name: "transit_gateway_attachment_id", Type: core.ConnectionTypeString, Label: "Transit Gateway Attachment ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	tgwID := strings.TrimSpace(awscommon.InputString("transit_gateway_id", inputs))
	if tgwID == "" {
		return nil, fmt.Errorf("transit_gateway_id is required")
	}
	peerTgwID := strings.TrimSpace(awscommon.InputString("peer_transit_gateway_id", inputs))
	if peerTgwID == "" {
		return nil, fmt.Errorf("peer_transit_gateway_id is required")
	}
	peerAccountID := strings.TrimSpace(awscommon.InputString("peer_account_id", inputs))
	if peerAccountID == "" {
		return nil, fmt.Errorf("peer_account_id is required")
	}
	peerRegion := strings.TrimSpace(awscommon.InputString("peer_region", inputs))
	if peerRegion == "" {
		return nil, fmt.Errorf("peer_region is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateTransitGatewayPeeringAttachmentInput{
		TransitGatewayId:     aws.String(tgwID),
		PeerTransitGatewayId: aws.String(peerTgwID),
		PeerAccountId:        aws.String(peerAccountID),
		PeerRegion:           aws.String(peerRegion),
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeTransitGatewayAttachment,
			Tags:         tags,
		}}
	}

	out, err := client.CreateTransitGatewayPeeringAttachment(ctx, in)
	if err != nil {
		return nil, err
	}

	attachment := map[string]interface{}{}
	id := ""
	if a := out.TransitGatewayPeeringAttachment; a != nil {
		id = aws.ToString(a.TransitGatewayAttachmentId)
		attachment = map[string]interface{}{
			"transit_gateway_attachment_id":          id,
			"accepter_transit_gateway_attachment_id": aws.ToString(a.AccepterTransitGatewayAttachmentId),
			"state":                                  string(a.State),
		}
		if r := a.RequesterTgwInfo; r != nil {
			attachment["requester_transit_gateway_id"] = aws.ToString(r.TransitGatewayId)
			attachment["requester_owner_id"] = aws.ToString(r.OwnerId)
			attachment["requester_region"] = aws.ToString(r.Region)
		}
		if p := a.AccepterTgwInfo; p != nil {
			attachment["accepter_transit_gateway_id"] = aws.ToString(p.TransitGatewayId)
			attachment["accepter_owner_id"] = aws.ToString(p.OwnerId)
			attachment["accepter_region"] = aws.ToString(p.Region)
		}
	}

	return map[string]interface{}{
		"tool_result":                   fmt.Sprintf("Created transit gateway peering attachment %s", id),
		"attachment":                    attachment,
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
