// Package aws_vpc_create_network_interface creates an elastic network
// interface (ENI) in a subnet.
package aws_vpc_create_network_interface

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
	Name         = "AWS VPC Create Network Interface"
	Description  = "Create an elastic network interface (ENI) in a subnet."
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
	{Name: "subnet_id", Type: core.ConnectionTypeString, Label: "Subnet ID", Placeholder: "subnet-0abc", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)", Placeholder: "e.g. App server ENI"},
	{Name: "private_ip_address", Type: core.ConnectionTypeString, Label: "Private IP Address (optional)", Placeholder: "e.g. 10.0.1.10"},
	{Name: "groups", Type: core.ConnectionTypeString, Label: "Security Group IDs (optional)", Placeholder: "Comma-separated, e.g. sg-0abc,sg-0def"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "network_interface", Type: core.ConnectionTypeObject, Label: "Network Interface"},
	{Name: "network_interface_id", Type: core.ConnectionTypeString, Label: "Network Interface ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	subnetID := strings.TrimSpace(awscommon.InputString("subnet_id", inputs))
	if subnetID == "" {
		return nil, fmt.Errorf("subnet_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.CreateNetworkInterfaceInput{SubnetId: aws.String(subnetID)}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if ip := strings.TrimSpace(awscommon.InputString("private_ip_address", inputs)); ip != "" {
		in.PrivateIpAddress = aws.String(ip)
	}
	if groups := awscommon.InputStrings("groups", inputs); len(groups) > 0 {
		in.Groups = groups
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.TagSpecifications = []ec2types.TagSpecification{{
			ResourceType: ec2types.ResourceTypeNetworkInterface,
			Tags:         tags,
		}}
	}

	out, err := client.CreateNetworkInterface(ctx, in)
	if err != nil {
		return nil, err
	}

	eni := map[string]interface{}{}
	id := ""
	if out.NetworkInterface != nil {
		n := out.NetworkInterface
		id = aws.ToString(n.NetworkInterfaceId)
		eni = map[string]interface{}{
			"network_interface_id": id,
			"subnet_id":            aws.ToString(n.SubnetId),
			"vpc_id":               aws.ToString(n.VpcId),
			"private_ip_address":   aws.ToString(n.PrivateIpAddress),
			"status":               string(n.Status),
			"availability_zone":    aws.ToString(n.AvailabilityZone),
		}
	}

	return map[string]interface{}{
		"tool_result":          fmt.Sprintf("Created network interface %s in %s", id, subnetID),
		"network_interface":    eni,
		"network_interface_id": id,
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
