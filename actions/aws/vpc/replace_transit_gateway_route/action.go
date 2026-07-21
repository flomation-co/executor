// Package aws_vpc_replace_transit_gateway_route replaces an existing route in a transit gateway route table.
package aws_vpc_replace_transit_gateway_route

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
	Name         = "AWS VPC Replace Transit Gateway Route"
	Description  = "Replace an existing route in a transit gateway route table."
	Website      = "https://www.flomation.co"
	Icon         = "route+pen"
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
	{Name: "transit_gateway_route_table_id", Type: core.ConnectionTypeString, Label: "Transit Gateway Route Table ID", Placeholder: "tgw-rtb-0123456789abcdef0", Required: true},
	{Name: "destination_cidr_block", Type: core.ConnectionTypeString, Label: "Destination CIDR Block", Placeholder: "10.1.0.0/16", Required: true},
	{Name: "transit_gateway_attachment_id", Type: core.ConnectionTypeString, Label: "Transit Gateway Attachment ID (optional)", Placeholder: "New target attachment; omit for a blackhole route"},
	{Name: "blackhole", Type: core.ConnectionTypeBoolean, Label: "Blackhole", Placeholder: "Drop traffic matching this route"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "route", Type: core.ConnectionTypeObject, Label: "Route"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	rtID := strings.TrimSpace(awscommon.InputString("transit_gateway_route_table_id", inputs))
	if rtID == "" {
		return nil, fmt.Errorf("transit_gateway_route_table_id is required")
	}
	cidr := strings.TrimSpace(awscommon.InputString("destination_cidr_block", inputs))
	if cidr == "" {
		return nil, fmt.Errorf("destination_cidr_block is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.ReplaceTransitGatewayRouteInput{
		TransitGatewayRouteTableId: aws.String(rtID),
		DestinationCidrBlock:       aws.String(cidr),
	}
	if attID := strings.TrimSpace(awscommon.InputString("transit_gateway_attachment_id", inputs)); attID != "" {
		in.TransitGatewayAttachmentId = aws.String(attID)
	}
	if awscommon.InputBool("blackhole", inputs) {
		in.Blackhole = aws.Bool(true)
	}

	out, err := client.ReplaceTransitGatewayRoute(ctx, in)
	if err != nil {
		return nil, err
	}

	route := map[string]interface{}{}
	if r := out.Route; r != nil {
		route = map[string]interface{}{
			"destination_cidr_block": aws.ToString(r.DestinationCidrBlock),
			"state":                  string(r.State),
			"type":                   string(r.Type),
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Replaced route %s in route table %s", cidr, rtID),
		"route":       route,
	}, nil
}
