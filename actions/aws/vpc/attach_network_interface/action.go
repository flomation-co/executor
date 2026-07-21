// Package aws_vpc_attach_network_interface attaches an ENI to an EC2 instance.
package aws_vpc_attach_network_interface

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
	Name         = "AWS VPC Attach Network Interface"
	Description  = "Attach an elastic network interface (ENI) to an EC2 instance."
	Website      = "https://www.flomation.co"
	Icon         = "server+link"
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
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Network Interface ID", Placeholder: "eni-0abc", Required: true},
	{Name: "instance_id", Type: core.ConnectionTypeString, Label: "Instance ID", Placeholder: "i-0abc", Required: true},
	{Name: "device_index", Type: core.ConnectionTypeInteger, Label: "Device Index", Placeholder: "e.g. 1 (0 is the primary interface)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attachment_id", Type: core.ConnectionTypeString, Label: "Attachment ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	eniID := strings.TrimSpace(awscommon.InputString("network_interface_id", inputs))
	if eniID == "" {
		return nil, fmt.Errorf("network_interface_id is required")
	}
	instanceID := strings.TrimSpace(awscommon.InputString("instance_id", inputs))
	if instanceID == "" {
		return nil, fmt.Errorf("instance_id is required")
	}
	idx, ok := awscommon.InputInt("device_index", inputs)
	if !ok {
		return nil, fmt.Errorf("device_index is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.AttachNetworkInterface(ctx, &ec2.AttachNetworkInterfaceInput{
		NetworkInterfaceId: aws.String(eniID),
		InstanceId:         aws.String(instanceID),
		DeviceIndex:        aws.Int32(int32(idx)),
	})
	if err != nil {
		return nil, err
	}

	attachmentID := aws.ToString(out.AttachmentId)

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Attached network interface %s to %s (attachment %s)", eniID, instanceID, attachmentID),
		"attachment_id": attachmentID,
	}, nil
}
