// Package aws_vpc_modify_vpn_connection_options changes the local/remote IPv4
// network CIDRs of a Site-to-Site VPN connection.
package aws_vpc_modify_vpn_connection_options

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
	Name         = "AWS VPC Modify VPN Connection Options"
	Description  = "Change the local/remote IPv4 network CIDRs of a Site-to-Site VPN connection."
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
	{Name: "local_ipv4_network_cidr", Type: core.ConnectionTypeString, Label: "Local IPv4 Network CIDR (optional)", Placeholder: "10.0.0.0/16 — customer gateway side"},
	{Name: "remote_ipv4_network_cidr", Type: core.ConnectionTypeString, Label: "Remote IPv4 Network CIDR (optional)", Placeholder: "172.16.0.0/16 — Amazon side"},
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
	localCidr := strings.TrimSpace(awscommon.InputString("local_ipv4_network_cidr", inputs))
	remoteCidr := strings.TrimSpace(awscommon.InputString("remote_ipv4_network_cidr", inputs))
	if localCidr == "" && remoteCidr == "" {
		return nil, fmt.Errorf("provide at least one of local_ipv4_network_cidr or remote_ipv4_network_cidr")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.ModifyVpnConnectionOptionsInput{VpnConnectionId: aws.String(vpnID)}
	if localCidr != "" {
		in.LocalIpv4NetworkCidr = aws.String(localCidr)
	}
	if remoteCidr != "" {
		in.RemoteIpv4NetworkCidr = aws.String(remoteCidr)
	}

	out, err := client.ModifyVpnConnectionOptions(ctx, in)
	if err != nil {
		return nil, err
	}

	// CustomerGatewayConfiguration (the opaque XML tunnel/pre-shared-key blob) is
	// deliberately excluded from the flatten. Retrieve it from the AWS console.
	conn := map[string]interface{}{}
	id := vpnID
	if out.VpnConnection != nil {
		id = aws.ToString(out.VpnConnection.VpnConnectionId)
		conn = map[string]interface{}{
			"vpn_connection_id":   id,
			"state":               string(out.VpnConnection.State),
			"type":                string(out.VpnConnection.Type),
			"customer_gateway_id": aws.ToString(out.VpnConnection.CustomerGatewayId),
		}
	}

	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Modified VPN connection options for %s", id),
		"vpn_connection":    conn,
		"vpn_connection_id": id,
	}, nil
}
