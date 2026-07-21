// Package aws_vpc_unassign_private_ip_addresses removes secondary private IPv4
// addresses from an elastic network interface (ENI).
package aws_vpc_unassign_private_ip_addresses

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
	Name         = "AWS VPC Unassign Private IP Addresses"
	Description  = "Remove secondary private IPv4 addresses from a network interface."
	Website      = "https://www.flomation.co"
	Icon         = "server+minus"
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
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Network Interface ID", Placeholder: "eni-0abc...", Required: true},
	{Name: "private_ip_addresses", Type: core.ConnectionTypeString, Label: "Private IP Addresses", Placeholder: "Comma-separated, e.g. 10.0.1.10,10.0.1.11", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Network Interface ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("network_interface_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("network_interface_id is required")
	}

	ips := awscommon.InputStrings("private_ip_addresses", inputs)
	if len(ips) == 0 {
		return nil, fmt.Errorf("at least one private IP address is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	if _, err := client.UnassignPrivateIpAddresses(ctx, &ec2.UnassignPrivateIpAddressesInput{
		NetworkInterfaceId: aws.String(id),
		PrivateIpAddresses: ips,
	}); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":          fmt.Sprintf("Unassigned %d private IP address(es) from %s", len(ips), id),
		"network_interface_id": id,
	}, nil
}
