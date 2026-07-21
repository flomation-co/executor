// Package aws_vpc_assign_ipv6_addresses assigns IPv6 addresses to an elastic
// network interface (ENI).
package aws_vpc_assign_ipv6_addresses

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
	Name         = "AWS VPC Assign IPv6 Addresses"
	Description  = "Assign IPv6 addresses to a network interface."
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
	{Name: "ipv6_addresses", Type: core.ConnectionTypeString, Label: "IPv6 Addresses (optional)", Placeholder: "Comma-separated, e.g. 2001:db8::1,2001:db8::2"},
	{Name: "ipv6_address_count", Type: core.ConnectionTypeInteger, Label: "IPv6 Address Count (optional)", Placeholder: "e.g. 2"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "assigned_ipv6_addresses", Type: core.ConnectionTypeObject, Label: "Assigned IPv6 Addresses"},
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Network Interface ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("network_interface_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("network_interface_id is required")
	}

	in := &ec2.AssignIpv6AddressesInput{NetworkInterfaceId: aws.String(id)}
	if addrs := awscommon.InputStrings("ipv6_addresses", inputs); len(addrs) > 0 {
		in.Ipv6Addresses = addrs
	}
	if count, ok := intInput("ipv6_address_count", inputs); ok {
		in.Ipv6AddressCount = aws.Int32(count)
	}
	if len(in.Ipv6Addresses) == 0 && in.Ipv6AddressCount == nil {
		return nil, fmt.Errorf("provide ipv6_addresses or ipv6_address_count")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.AssignIpv6Addresses(ctx, in)
	if err != nil {
		return nil, err
	}

	assigned := []string{}
	if out != nil {
		assigned = append(assigned, out.AssignedIpv6Addresses...)
	}

	return map[string]interface{}{
		"tool_result":             fmt.Sprintf("Assigned %d IPv6 address(es) to %s", len(assigned), id),
		"assigned_ipv6_addresses": assigned,
		"network_interface_id":    id,
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
