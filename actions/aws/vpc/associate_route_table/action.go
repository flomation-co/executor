// Package aws_vpc_associate_route_table associates a route table with a subnet or gateway.
package aws_vpc_associate_route_table

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
	Name         = "AWS VPC Associate Route Table"
	Description  = "Associate a route table with a subnet or gateway."
	Website      = "https://www.flomation.co"
	Icon         = "route+link"
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
	{Name: "subnet_id", Type: core.ConnectionTypeString, Label: "Subnet ID (optional)", Placeholder: "subnet-0abc..."},
	{Name: "gateway_id", Type: core.ConnectionTypeString, Label: "Gateway ID (optional)", Placeholder: "igw-0abc..."},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "association_id", Type: core.ConnectionTypeString, Label: "Association ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	rtID := strings.TrimSpace(awscommon.InputString("route_table_id", inputs))
	if rtID == "" {
		return nil, fmt.Errorf("route_table_id is required")
	}
	subnetID := strings.TrimSpace(awscommon.InputString("subnet_id", inputs))
	gatewayID := strings.TrimSpace(awscommon.InputString("gateway_id", inputs))
	if subnetID == "" && gatewayID == "" {
		return nil, fmt.Errorf("provide a subnet_id or a gateway_id to associate")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.AssociateRouteTableInput{RouteTableId: aws.String(rtID)}
	if subnetID != "" {
		in.SubnetId = aws.String(subnetID)
	}
	if gatewayID != "" {
		in.GatewayId = aws.String(gatewayID)
	}

	out, err := client.AssociateRouteTable(ctx, in)
	if err != nil {
		return nil, err
	}

	assocID := aws.ToString(out.AssociationId)
	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Associated route table %s (association %s)", rtID, assocID),
		"association_id": assocID,
	}, nil
}
