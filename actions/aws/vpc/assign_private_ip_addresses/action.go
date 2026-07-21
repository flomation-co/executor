// Package aws_vpc_assign_private_ip_addresses assigns secondary private IPv4
// addresses to an elastic network interface (ENI).
package aws_vpc_assign_private_ip_addresses

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
	Name         = "AWS VPC Assign Private IP Addresses"
	Description  = "Assign secondary private IPv4 addresses to a network interface."
	Website      = "https://www.flomation.co"
	Icon         = "server+plus"
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
	{Name: "private_ip_addresses", Type: core.ConnectionTypeString, Label: "Private IP Addresses (optional)", Placeholder: "Comma-separated, e.g. 10.0.1.10,10.0.1.11"},
	{Name: "secondary_private_ip_address_count", Type: core.ConnectionTypeInteger, Label: "Secondary IP Count (optional)", Placeholder: "e.g. 2"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "assigned_private_ip_addresses", Type: core.ConnectionTypeObject, Label: "Assigned Private IP Addresses"},
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Network Interface ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("network_interface_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("network_interface_id is required")
	}

	in := &ec2.AssignPrivateIpAddressesInput{NetworkInterfaceId: aws.String(id)}
	if ips := awscommon.InputStrings("private_ip_addresses", inputs); len(ips) > 0 {
		in.PrivateIpAddresses = ips
	}
	if count, ok := intInput("secondary_private_ip_address_count", inputs); ok {
		in.SecondaryPrivateIpAddressCount = aws.Int32(count)
	}
	if len(in.PrivateIpAddresses) == 0 && in.SecondaryPrivateIpAddressCount == nil {
		return nil, fmt.Errorf("provide private_ip_addresses or secondary_private_ip_address_count")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.AssignPrivateIpAddresses(ctx, in)
	if err != nil {
		return nil, err
	}

	assigned := []string{}
	if out != nil {
		for _, a := range out.AssignedPrivateIpAddresses {
			assigned = append(assigned, aws.ToString(a.PrivateIpAddress))
		}
	}

	return map[string]interface{}{
		"tool_result":                   fmt.Sprintf("Assigned %d private IP address(es) to %s", len(assigned), id),
		"assigned_private_ip_addresses": assigned,
		"network_interface_id":          id,
	}, nil
}

func intInput(name string, inputs []*core.Connection) (int32, bool) {
	c := core.FindConnection(name, inputs)
	if c == nil {
		return 0, false
	}
	n := c.Number()
	if n == nil {
		return 0, false
	}
	return int32(*n), true
}
