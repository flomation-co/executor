// Package aws_vpc_create_route adds a route to a route table.
package aws_vpc_create_route

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
	Name         = "AWS VPC Create Route"
	Description  = "Add a route to a route table pointing at a gateway or other target."
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
	{Name: "route_table_id", Type: core.ConnectionTypeString, Label: "Route Table ID", Placeholder: "rtb-0abc...", Required: true},
	{Name: "destination_cidr_block", Type: core.ConnectionTypeString, Label: "Destination CIDR Block", Placeholder: "0.0.0.0/0", Required: true},
	{Name: "target_type", Type: core.ConnectionTypeString, Label: "Target Type", Required: true, Options: []core.ConnectionOption{
		{Name: "Internet Gateway", Value: "internet_gateway"},
		{Name: "NAT Gateway", Value: "nat_gateway"},
		{Name: "Network Interface", Value: "network_interface"},
		{Name: "VPC Peering Connection", Value: "vpc_peering"},
		{Name: "Transit Gateway", Value: "transit_gateway"},
		{Name: "Egress-only Internet Gateway", Value: "egress_only_igw"},
	}},
	{Name: "target_id", Type: core.ConnectionTypeString, Label: "Target ID", Placeholder: "igw-0abc... / nat-... / eni-... etc", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	rtID := strings.TrimSpace(awscommon.InputString("route_table_id", inputs))
	if rtID == "" {
		return nil, fmt.Errorf("route_table_id is required")
	}
	cidr := strings.TrimSpace(awscommon.InputString("destination_cidr_block", inputs))
	if cidr == "" {
		return nil, fmt.Errorf("destination_cidr_block is required")
	}
	targetType := strings.TrimSpace(awscommon.InputString("target_type", inputs))
	if targetType == "" {
		return nil, fmt.Errorf("target_type is required")
	}
	targetID := strings.TrimSpace(awscommon.InputString("target_id", inputs))
	if targetID == "" {
		return nil, fmt.Errorf("target_id is required")
	}

	in := &ec2.CreateRouteInput{
		RouteTableId:         aws.String(rtID),
		DestinationCidrBlock: aws.String(cidr),
	}
	switch targetType {
	case "internet_gateway":
		in.GatewayId = aws.String(targetID)
	case "nat_gateway":
		in.NatGatewayId = aws.String(targetID)
	case "network_interface":
		in.NetworkInterfaceId = aws.String(targetID)
	case "vpc_peering":
		in.VpcPeeringConnectionId = aws.String(targetID)
	case "transit_gateway":
		in.TransitGatewayId = aws.String(targetID)
	case "egress_only_igw":
		in.EgressOnlyInternetGatewayId = aws.String(targetID)
	default:
		return nil, fmt.Errorf("unsupported target_type %q", targetType)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	if _, err := client.CreateRoute(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created route %s → %s (%s) in %s", cidr, targetID, targetType, rtID),
	}, nil
}
