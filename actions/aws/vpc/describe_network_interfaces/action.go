// Package aws_vpc_describe_network_interfaces lists elastic network interfaces.
package aws_vpc_describe_network_interfaces

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
	Name         = "AWS VPC Describe Network Interfaces"
	Description  = "List elastic network interfaces (ENIs), optionally by id, subnet or tags."
	Website      = "https://www.flomation.co"
	Icon         = "server+magnifying-glass"
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
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Network Interface IDs (optional)", Placeholder: "Comma-separated; blank lists all"},
	{Name: "subnet_id", Type: core.ConnectionTypeString, Label: "Filter by Subnet ID (optional)", Placeholder: "subnet-0abc"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags", Placeholder: "Only return interfaces with these tags (blank Value matches any value for that key)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "network_interfaces", Type: core.ConnectionTypeObject, Label: "Network Interfaces"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeNetworkInterfacesInput{}
	if ids := awscommon.InputStrings("network_interface_id", inputs); len(ids) > 0 {
		in.NetworkInterfaceIds = ids
	}
	if filters := awscommon.BuildEC2Filters(inputs, []awscommon.FilterSpec{
		{Input: "subnet_id", Filter: "subnet-id"},
	}); len(filters) > 0 {
		in.Filters = filters
	}

	var interfaces []map[string]interface{}
	paginator := ec2.NewDescribeNetworkInterfacesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, n := range page.NetworkInterfaces {
			attachmentID := ""
			instanceID := ""
			if n.Attachment != nil {
				attachmentID = aws.ToString(n.Attachment.AttachmentId)
				instanceID = aws.ToString(n.Attachment.InstanceId)
			}
			interfaces = append(interfaces, map[string]interface{}{
				"network_interface_id": aws.ToString(n.NetworkInterfaceId),
				"subnet_id":            aws.ToString(n.SubnetId),
				"vpc_id":               aws.ToString(n.VpcId),
				"private_ip_address":   aws.ToString(n.PrivateIpAddress),
				"status":               string(n.Status),
				"interface_type":       string(n.InterfaceType),
				"attachment_id":        attachmentID,
				"instance_id":          instanceID,
			})
		}
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Found %d network interface(s)", len(interfaces)),
		"network_interfaces": interfaces,
		"count":              len(interfaces),
	}, nil
}
