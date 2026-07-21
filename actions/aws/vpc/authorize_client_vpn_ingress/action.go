// Package aws_vpc_authorize_client_vpn_ingress adds an ingress authorization rule
// to an AWS Client VPN endpoint.
package aws_vpc_authorize_client_vpn_ingress

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
	Name         = "AWS VPC Authorize Client VPN Ingress"
	Description  = "Add an ingress authorization rule to an AWS Client VPN endpoint."
	Website      = "https://www.flomation.co"
	Icon         = "key+plus"
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
	{Name: "client_vpn_endpoint_id", Type: core.ConnectionTypeString, Label: "Client VPN Endpoint ID", Placeholder: "cvpn-endpoint-0abc123", Required: true},
	{Name: "target_network_cidr", Type: core.ConnectionTypeString, Label: "Target Network CIDR", Placeholder: "10.0.0.0/16", Required: true},
	{Name: "authorize_all_groups", Type: core.ConnectionTypeBoolean, Label: "Authorize All Groups (optional)"},
	{Name: "access_group_id", Type: core.ConnectionTypeString, Label: "Access Group ID (optional)", Placeholder: "Required unless authorizing all groups"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	endpointID := strings.TrimSpace(awscommon.InputString("client_vpn_endpoint_id", inputs))
	if endpointID == "" {
		return nil, fmt.Errorf("client_vpn_endpoint_id is required")
	}
	targetCIDR := strings.TrimSpace(awscommon.InputString("target_network_cidr", inputs))
	if targetCIDR == "" {
		return nil, fmt.Errorf("target_network_cidr is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.AuthorizeClientVpnIngressInput{
		ClientVpnEndpointId: aws.String(endpointID),
		TargetNetworkCidr:   aws.String(targetCIDR),
	}
	if awscommon.InputBool("authorize_all_groups", inputs) {
		in.AuthorizeAllGroups = aws.Bool(true)
	}
	if group := strings.TrimSpace(awscommon.InputString("access_group_id", inputs)); group != "" {
		in.AccessGroupId = aws.String(group)
	}
	if desc := strings.TrimSpace(awscommon.InputString("description", inputs)); desc != "" {
		in.Description = aws.String(desc)
	}

	out, err := client.AuthorizeClientVpnIngress(ctx, in)
	if err != nil {
		return nil, err
	}

	status := ""
	if out.Status != nil {
		status = string(out.Status.Code)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Authorized ingress to %s on Client VPN endpoint %s (status=%s)", targetCIDR, endpointID, status),
		"status":      status,
	}, nil
}
