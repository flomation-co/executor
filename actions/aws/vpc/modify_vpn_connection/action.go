// Package aws_vpc_modify_vpn_connection changes the target gateway of an
// existing Site-to-Site VPN connection.
package aws_vpc_modify_vpn_connection

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
	Name         = "AWS VPC Modify VPN Connection"
	Description  = "Change the target gateway (transit, VPN, or customer) of a Site-to-Site VPN connection."
	Website      = "https://www.flomation.co"
	Icon         = "lock+pen"
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
	{Name: "vpn_connection_id", Type: core.ConnectionTypeString, Label: "VPN Connection ID", Placeholder: "vpn-0abc123", Required: true},
	{Name: "transit_gateway_id", Type: core.ConnectionTypeString, Label: "Transit Gateway ID (optional)", Placeholder: "tgw-0abc123"},
	{Name: "vpn_gateway_id", Type: core.ConnectionTypeString, Label: "VPN Gateway ID (optional)", Placeholder: "vgw-0abc123"},
	{Name: "customer_gateway_id", Type: core.ConnectionTypeString, Label: "Customer Gateway ID (optional)", Placeholder: "cgw-0abc123"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vpn_connection", Type: core.ConnectionTypeObject, Label: "VPN Connection"},
	{Name: "vpn_connection_id", Type: core.ConnectionTypeString, Label: "VPN Connection ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	vpnID := strings.TrimSpace(awscommon.InputString("vpn_connection_id", inputs))
	if vpnID == "" {
		return nil, fmt.Errorf("vpn_connection_id is required")
	}
	tgwID := strings.TrimSpace(awscommon.InputString("transit_gateway_id", inputs))
	vgwID := strings.TrimSpace(awscommon.InputString("vpn_gateway_id", inputs))
	cgwID := strings.TrimSpace(awscommon.InputString("customer_gateway_id", inputs))
	if tgwID == "" && vgwID == "" && cgwID == "" {
		return nil, fmt.Errorf("provide at least one of transit_gateway_id, vpn_gateway_id, or customer_gateway_id")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.ModifyVpnConnectionInput{VpnConnectionId: aws.String(vpnID)}
	if tgwID != "" {
		in.TransitGatewayId = aws.String(tgwID)
	}
	if vgwID != "" {
		in.VpnGatewayId = aws.String(vgwID)
	}
	if cgwID != "" {
		in.CustomerGatewayId = aws.String(cgwID)
	}

	out, err := client.ModifyVpnConnection(ctx, in)
	if err != nil {
		return nil, err
	}

	// The customer gateway configuration (tunnels, pre-shared keys) is a large,
	// opaque XML provider blob in CustomerGatewayConfiguration — deliberately
	// excluded from the flatten. Retrieve it from the AWS console if needed.
	conn := map[string]interface{}{}
	id := vpnID
	if out.VpnConnection != nil {
		id = aws.ToString(out.VpnConnection.VpnConnectionId)
		conn = map[string]interface{}{
			"vpn_connection_id":   id,
			"state":               string(out.VpnConnection.State),
			"type":                string(out.VpnConnection.Type),
			"customer_gateway_id": aws.ToString(out.VpnConnection.CustomerGatewayId),
			"vpn_gateway_id":      aws.ToString(out.VpnConnection.VpnGatewayId),
			"transit_gateway_id":  aws.ToString(out.VpnConnection.TransitGatewayId),
		}
	}

	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Modified VPN connection %s", id),
		"vpn_connection":    conn,
		"vpn_connection_id": id,
	}, nil
}
