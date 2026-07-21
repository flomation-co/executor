// Package aws_vpc_create_vpn_connection creates a Site-to-Site VPN connection
// between a customer gateway and a virtual private (or transit) gateway.
package aws_vpc_create_vpn_connection

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
	Name         = "AWS VPC Create VPN Connection"
	Description  = "Create a Site-to-Site VPN connection between a customer gateway and an Amazon gateway."
	Website      = "https://www.flomation.co"
	Icon         = "lock+plus"
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
	{Name: "type", Type: core.ConnectionTypeString, Label: "VPN Type", Required: true, Options: []core.ConnectionOption{
		{Name: "ipsec.1", Value: "ipsec.1"},
	}},
	{Name: "customer_gateway_id", Type: core.ConnectionTypeString, Label: "Customer Gateway ID", Placeholder: "cgw-0abc123", Required: true},
	{Name: "vpn_gateway_id", Type: core.ConnectionTypeString, Label: "VPN Gateway ID (optional)", Placeholder: "vgw-0abc123 — provide this OR a transit gateway"},
	{Name: "transit_gateway_id", Type: core.ConnectionTypeString, Label: "Transit Gateway ID (optional)", Placeholder: "tgw-0abc123 — provide this OR a VPN gateway"},
	{Name: "static_routes_only", Type: core.ConnectionTypeBoolean, Label: "Static Routes Only"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vpn_connection", Type: core.ConnectionTypeObject, Label: "VPN Connection"},
	{Name: "vpn_connection_id", Type: core.ConnectionTypeString, Label: "VPN Connection ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	vpnType := strings.TrimSpace(awscommon.InputString("type", inputs))
	if vpnType == "" {
		return nil, fmt.Errorf("type is required")
	}
	cgwID := strings.TrimSpace(awscommon.InputString("customer_gateway_id", inputs))
	if cgwID == "" {
		return nil, fmt.Errorf("customer_gateway_id is required")
	}
	vgwID := strings.TrimSpace(awscommon.InputString("vpn_gateway_id", inputs))
	tgwID := strings.TrimSpace(awscommon.InputString("transit_gateway_id", inputs))
	if vgwID == "" && tgwID == "" {
		return nil, fmt.Errorf("provide either vpn_gateway_id or transit_gateway_id")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateVpnConnectionInput{
		Type:              aws.String(vpnType),
		CustomerGatewayId: aws.String(cgwID),
	}
	if vgwID != "" {
		in.VpnGatewayId = aws.String(vgwID)
	}
	if tgwID != "" {
		in.TransitGatewayId = aws.String(tgwID)
	}
	if awscommon.InputBool("static_routes_only", inputs) {
		in.Options = &ec2types.VpnConnectionOptionsSpecification{StaticRoutesOnly: aws.Bool(true)}
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeVpnConnection,
			Tags:         tags,
		}}
	}

	out, err := client.CreateVpnConnection(ctx, in)
	if err != nil {
		return nil, err
	}

	// The customer gateway configuration (tunnels, pre-shared keys) is a large,
	// opaque XML provider blob in CustomerGatewayConfiguration — deliberately
	// excluded from the flatten. Retrieve it from the AWS console if needed.
	conn := map[string]interface{}{}
	id := ""
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
		"tool_result":       fmt.Sprintf("Created VPN connection %s (tunnel config is available in the AWS console)", id),
		"vpn_connection":    conn,
		"vpn_connection_id": id,
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
