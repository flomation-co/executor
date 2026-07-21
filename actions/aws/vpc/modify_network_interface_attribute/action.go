// Package aws_vpc_modify_network_interface_attribute modifies attributes of an
// elastic network interface (ENI): description, source/dest check, and security
// groups.
package aws_vpc_modify_network_interface_attribute

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
	Name         = "AWS VPC Modify Network Interface Attribute"
	Description  = "Modify an ENI's description, source/dest check, or security groups."
	Website      = "https://www.flomation.co"
	Icon         = "server+pen"
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
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description (optional)", Placeholder: "e.g. App server ENI"},
	{Name: "source_dest_check", Type: core.ConnectionTypeString, Label: "Source/Dest Check", Options: []core.ConnectionOption{
		{Name: "Leave unchanged", Value: ""},
		{Name: "Enable", Value: "true"},
		{Name: "Disable", Value: "false"},
	}},
	{Name: "groups", Type: core.ConnectionTypeString, Label: "Security Group IDs (optional)", Placeholder: "Comma-separated, e.g. sg-0abc,sg-0def"},
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

	in := &ec2.ModifyNetworkInterfaceAttributeInput{NetworkInterfaceId: aws.String(id)}
	var changed []string

	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = &ec2types.AttributeValue{Value: aws.String(d)}
		changed = append(changed, "description")
	}
	if check := triState("source_dest_check", inputs); check != nil {
		in.SourceDestCheck = &ec2types.AttributeBooleanValue{Value: aws.Bool(*check)}
		changed = append(changed, fmt.Sprintf("source_dest_check=%t", *check))
	}
	if groups := awscommon.InputStrings("groups", inputs); len(groups) > 0 {
		in.Groups = groups
		changed = append(changed, fmt.Sprintf("groups=%d", len(groups)))
	}

	if len(changed) == 0 {
		return nil, fmt.Errorf("set at least one of description, source_dest_check or groups")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	if _, err := client.ModifyNetworkInterfaceAttribute(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":          fmt.Sprintf("Modified network interface %s (%s)", id, strings.Join(changed, ", ")),
		"network_interface_id": id,
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
