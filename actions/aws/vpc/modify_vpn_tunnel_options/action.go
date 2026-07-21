// Package aws_vpc_modify_vpn_tunnel_options modifies the IPSec options of a
// single VPN tunnel within a Site-to-Site VPN connection.
package aws_vpc_modify_vpn_tunnel_options

import (
	"context"
	"encoding/json"
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
	Name         = "AWS VPC Modify VPN Tunnel Options"
	Description  = "Modify the IPSec options of a single tunnel in a Site-to-Site VPN connection."
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
	{Name: "vpn_tunnel_outside_ip_address", Type: core.ConnectionTypeString, Label: "Tunnel Outside IP Address", Placeholder: "203.0.113.10 — external IP of the tunnel to modify", Required: true},
	{Name: "tunnel_options", Type: core.ConnectionTypeText, Label: "Tunnel Options", Placeholder: `JSON object e.g. {"pre_shared_key":"...","phase1_lifetime_seconds":28800,"phase2_lifetime_seconds":3600}`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vpn_connection", Type: core.ConnectionTypeObject, Label: "VPN Connection"},
	{Name: "vpn_connection_id", Type: core.ConnectionTypeString, Label: "VPN Connection ID"},
}

// tunnelOptionsInput is the minimal, safe subset of AWS's large
// ModifyVpnTunnelOptionsSpecification. Only the pre-shared key and the phase 1/2
// lifetimes are wired; the many algorithm-list and DPD/rekey fields are omitted
// for v1 — use the AWS console for those.
type tunnelOptionsInput struct {
	PreSharedKey          string `json:"pre_shared_key"`
	Phase1LifetimeSeconds *int32 `json:"phase1_lifetime_seconds"`
	Phase2LifetimeSeconds *int32 `json:"phase2_lifetime_seconds"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	vpnID := strings.TrimSpace(awscommon.InputString("vpn_connection_id", inputs))
	if vpnID == "" {
		return nil, fmt.Errorf("vpn_connection_id is required")
	}
	outsideIP := strings.TrimSpace(awscommon.InputString("vpn_tunnel_outside_ip_address", inputs))
	if outsideIP == "" {
		return nil, fmt.Errorf("vpn_tunnel_outside_ip_address is required")
	}

	raw := strings.TrimSpace(awscommon.InputString("tunnel_options", inputs))
	if raw == "" {
		return nil, fmt.Errorf("tunnel_options is required")
	}
	var parsed tunnelOptionsInput
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, fmt.Errorf("tunnel_options must be a JSON object: %w", err)
	}

	spec := &ec2types.ModifyVpnTunnelOptionsSpecification{}
	if v := strings.TrimSpace(parsed.PreSharedKey); v != "" {
		spec.PreSharedKey = aws.String(v)
	}
	if parsed.Phase1LifetimeSeconds != nil {
		spec.Phase1LifetimeSeconds = parsed.Phase1LifetimeSeconds
	}
	if parsed.Phase2LifetimeSeconds != nil {
		spec.Phase2LifetimeSeconds = parsed.Phase2LifetimeSeconds
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.ModifyVpnTunnelOptionsInput{
		VpnConnectionId:           aws.String(vpnID),
		VpnTunnelOutsideIpAddress: aws.String(outsideIP),
		TunnelOptions:             spec,
	}

	out, err := client.ModifyVpnTunnelOptions(ctx, in)
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
		"tool_result":       fmt.Sprintf("Modified tunnel %s options for VPN connection %s", outsideIP, id),
		"vpn_connection":    conn,
		"vpn_connection_id": id,
	}, nil
}
