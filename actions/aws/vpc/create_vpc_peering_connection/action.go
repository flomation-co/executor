// Package aws_vpc_create_vpc_peering_connection requests a VPC peering
// connection between two VPCs.
package aws_vpc_create_vpc_peering_connection

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
	Name         = "AWS VPC Create Peering Connection"
	Description  = "Request a VPC peering connection between two VPCs (same or cross account/region)."
	Website      = "https://www.flomation.co"
	Icon         = "link+plus"
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
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "Requester VPC ID", Placeholder: "vpc-0abc", Required: true},
	{Name: "peer_vpc_id", Type: core.ConnectionTypeString, Label: "Peer (Accepter) VPC ID", Placeholder: "vpc-0def", Required: true},
	{Name: "peer_owner_id", Type: core.ConnectionTypeString, Label: "Peer Owner Account ID (optional)", Placeholder: "Leave blank for same account"},
	{Name: "peer_region", Type: core.ConnectionTypeString, Label: "Peer Region (optional)", Placeholder: "Leave blank for same region"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vpc_peering_connection", Type: core.ConnectionTypeObject, Label: "VPC Peering Connection"},
	{Name: "vpc_peering_connection_id", Type: core.ConnectionTypeString, Label: "VPC Peering Connection ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	vpcID := strings.TrimSpace(awscommon.InputString("vpc_id", inputs))
	if vpcID == "" {
		return nil, fmt.Errorf("vpc_id is required")
	}
	peerVPCID := strings.TrimSpace(awscommon.InputString("peer_vpc_id", inputs))
	if peerVPCID == "" {
		return nil, fmt.Errorf("peer_vpc_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateVpcPeeringConnectionInput{
		VpcId:     aws.String(vpcID),
		PeerVpcId: aws.String(peerVPCID),
	}
	if v := strings.TrimSpace(awscommon.InputString("peer_owner_id", inputs)); v != "" {
		in.PeerOwnerId = aws.String(v)
	}
	if v := strings.TrimSpace(awscommon.InputString("peer_region", inputs)); v != "" {
		in.PeerRegion = aws.String(v)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeVpcPeeringConnection,
			Tags:         tags,
		}}
	}

	out, err := client.CreateVpcPeeringConnection(ctx, in)
	if err != nil {
		return nil, err
	}

	conn := map[string]interface{}{}
	id := ""
	if out.VpcPeeringConnection != nil {
		pc := out.VpcPeeringConnection
		id = aws.ToString(pc.VpcPeeringConnectionId)
		conn = map[string]interface{}{
			"vpc_peering_connection_id": id,
			"status":                    statusCode(pc.Status),
			"requester_vpc_id":          vpcInfoID(pc.RequesterVpcInfo),
			"accepter_vpc_id":           vpcInfoID(pc.AccepterVpcInfo),
		}
	}

	return map[string]interface{}{
		"tool_result":               fmt.Sprintf("Requested VPC peering connection %s (%s → %s)", id, vpcID, peerVPCID),
		"vpc_peering_connection":    conn,
		"vpc_peering_connection_id": id,
	}, nil
}

func statusCode(s *ec2types.VpcPeeringConnectionStateReason) string {
	if s == nil {
		return ""
	}
	return string(s.Code)
}

func vpcInfoID(v *ec2types.VpcPeeringConnectionVpcInfo) string {
	if v == nil {
		return ""
	}
	return aws.ToString(v.VpcId)
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
