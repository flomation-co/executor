// Package aws_vpc_associate_address associates an Elastic IP with an instance or interface.
package aws_vpc_associate_address

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Associate Address"
	Description  = "Associate an Elastic IP with an instance or network interface."
	Website      = "https://www.flomation.co"
	Icon         = "link+arrow-right-arrow-left"
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
	{Name: "allocation_id", Type: core.ConnectionTypeString, Label: "Allocation ID", Placeholder: "eipalloc-0abc123", Required: true},
	{Name: "instance_id", Type: core.ConnectionTypeString, Label: "Instance ID", Placeholder: "i-0abc123 (instance or network interface required)"},
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Network Interface ID", Placeholder: "eni-0abc123 (instance or network interface required)"},
	{Name: "private_ip_address", Type: core.ConnectionTypeString, Label: "Private IP Address", Placeholder: "Optional, when the interface has multiple private IPs"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "association_id", Type: core.ConnectionTypeString, Label: "Association ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	allocationID := awscommon.InputString("allocation_id", inputs)
	if allocationID == "" {
		return nil, fmt.Errorf("allocation id is required")
	}
	instanceID := awscommon.InputString("instance_id", inputs)
	networkInterfaceID := awscommon.InputString("network_interface_id", inputs)
	if instanceID == "" && networkInterfaceID == "" {
		return nil, fmt.Errorf("provide an instance id or a network interface id")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.AssociateAddressInput{AllocationId: aws.String(allocationID)}
	if instanceID != "" {
		in.InstanceId = aws.String(instanceID)
	}
	if networkInterfaceID != "" {
		in.NetworkInterfaceId = aws.String(networkInterfaceID)
	}
	if v := awscommon.InputString("private_ip_address", inputs); v != "" {
		in.PrivateIpAddress = aws.String(v)
	}

	out, err := client.AssociateAddress(ctx, in)
	if err != nil {
		return nil, err
	}

	associationID := aws.ToString(out.AssociationId)

	return map[string]interface{}{
		"tool_result":    "Associated Elastic IP " + allocationID + " (" + associationID + ")",
		"association_id": associationID,
	}, nil
}
