// Package aws_vpc_associate_client_vpn_target_network associates a target subnet
// with an AWS Client VPN endpoint.
package aws_vpc_associate_client_vpn_target_network

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
	Name         = "AWS VPC Associate Client VPN Target Network"
	Description  = "Associate a target subnet with an AWS Client VPN endpoint."
	Website      = "https://www.flomation.co"
	Icon         = "key+link"
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
	{Name: "subnet_id", Type: core.ConnectionTypeString, Label: "Subnet ID", Placeholder: "subnet-0abc123", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "association_id", Type: core.ConnectionTypeString, Label: "Association ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	endpointID := strings.TrimSpace(awscommon.InputString("client_vpn_endpoint_id", inputs))
	if endpointID == "" {
		return nil, fmt.Errorf("client_vpn_endpoint_id is required")
	}
	subnetID := strings.TrimSpace(awscommon.InputString("subnet_id", inputs))
	if subnetID == "" {
		return nil, fmt.Errorf("subnet_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.AssociateClientVpnTargetNetwork(ctx, &ec2.AssociateClientVpnTargetNetworkInput{
		ClientVpnEndpointId: aws.String(endpointID),
		SubnetId:            aws.String(subnetID),
	})
	if err != nil {
		return nil, err
	}

	assocID := aws.ToString(out.AssociationId)
	status := ""
	if out.Status != nil {
		status = string(out.Status.Code)
	}

	return map[string]interface{}{
		"tool_result":    fmt.Sprintf("Associated subnet %s with Client VPN endpoint %s (association %s)", subnetID, endpointID, assocID),
		"association_id": assocID,
		"status":         status,
	}, nil
}
