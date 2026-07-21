// Package aws_vpc_modify_subnet_attribute toggles subnet auto-assign attributes.
package aws_vpc_modify_subnet_attribute

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
	Name         = "AWS VPC Modify Subnet Attribute"
	Description  = "Toggle auto-assign public IPv4 or IPv6 address on launch for a subnet."
	Website      = "https://www.flomation.co"
	Icon         = "object-group+pen"
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
	{Name: "subnet_id", Type: core.ConnectionTypeString, Label: "Subnet ID", Placeholder: "subnet-0abc...", Required: true},
	{Name: "map_public_ip_on_launch", Type: core.ConnectionTypeString, Label: "Auto-assign Public IPv4 on Launch", Options: []core.ConnectionOption{
		{Name: "Leave unchanged", Value: ""},
		{Name: "Enable", Value: "true"},
		{Name: "Disable", Value: "false"},
	}},
	{Name: "assign_ipv6_address_on_creation", Type: core.ConnectionTypeString, Label: "Auto-assign IPv6 on Creation", Options: []core.ConnectionOption{
		{Name: "Leave unchanged", Value: ""},
		{Name: "Enable", Value: "true"},
		{Name: "Disable", Value: "false"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := strings.TrimSpace(awscommon.InputString("subnet_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("subnet_id is required")
	}

	mapPublic := triState("map_public_ip_on_launch", inputs)
	assignIpv6 := triState("assign_ipv6_address_on_creation", inputs)
	if mapPublic == nil && assignIpv6 == nil {
		return nil, fmt.Errorf("set at least one of map_public_ip_on_launch or assign_ipv6_address_on_creation")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	// ModifySubnetAttribute accepts one attribute per call.
	var changed []string
	if mapPublic != nil {
		if _, err := client.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
			SubnetId:            aws.String(id),
			MapPublicIpOnLaunch: &ec2types.AttributeBooleanValue{Value: aws.Bool(*mapPublic)},
		}); err != nil {
			return nil, err
		}
		changed = append(changed, fmt.Sprintf("map_public_ip_on_launch=%t", *mapPublic))
	}
	if assignIpv6 != nil {
		if _, err := client.ModifySubnetAttribute(ctx, &ec2.ModifySubnetAttributeInput{
			SubnetId:                    aws.String(id),
			AssignIpv6AddressOnCreation: &ec2types.AttributeBooleanValue{Value: aws.Bool(*assignIpv6)},
		}); err != nil {
			return nil, err
		}
		changed = append(changed, fmt.Sprintf("assign_ipv6_address_on_creation=%t", *assignIpv6))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified subnet %s (%s)", id, strings.Join(changed, ", ")),
	}, nil
}

// triState reads an optional ""/"true"/"false" dropdown into a *bool. A blank or
// absent value returns nil (leave unchanged).
func triState(name string, inputs []*core.Connection) *bool {
	v := strings.TrimSpace(awscommon.InputString(name, inputs))
	switch v {
	case "true":
		b := true
		return &b
	case "false":
		b := false
		return &b
	}
	return nil
}
