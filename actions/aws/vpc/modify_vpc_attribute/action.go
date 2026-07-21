// Package aws_vpc_modify_vpc_attribute toggles DNS support/hostnames on a VPC.
package aws_vpc_modify_vpc_attribute

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
	Name         = "AWS VPC Modify VPC Attribute"
	Description  = "Enable or disable DNS support and DNS hostnames on a VPC."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+pen"
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
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID", Placeholder: "vpc-0abc...", Required: true},
	{Name: "enable_dns_support", Type: core.ConnectionTypeString, Label: "Enable DNS Support", Options: []core.ConnectionOption{
		{Name: "Leave unchanged", Value: ""},
		{Name: "Enable", Value: "true"},
		{Name: "Disable", Value: "false"},
	}},
	{Name: "enable_dns_hostnames", Type: core.ConnectionTypeString, Label: "Enable DNS Hostnames", Options: []core.ConnectionOption{
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

	id := strings.TrimSpace(awscommon.InputString("vpc_id", inputs))
	if id == "" {
		return nil, fmt.Errorf("vpc_id is required")
	}

	support := triState("enable_dns_support", inputs)
	hostnames := triState("enable_dns_hostnames", inputs)
	if support == nil && hostnames == nil {
		return nil, fmt.Errorf("set at least one of enable_dns_support or enable_dns_hostnames")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	// ModifyVpcAttribute accepts one attribute per call, so make a separate call
	// for each provided value.
	var changed []string
	if support != nil {
		if _, err := client.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
			VpcId:            aws.String(id),
			EnableDnsSupport: &ec2types.AttributeBooleanValue{Value: aws.Bool(*support)},
		}); err != nil {
			return nil, err
		}
		changed = append(changed, fmt.Sprintf("dns_support=%t", *support))
	}
	if hostnames != nil {
		if _, err := client.ModifyVpcAttribute(ctx, &ec2.ModifyVpcAttributeInput{
			VpcId:              aws.String(id),
			EnableDnsHostnames: &ec2types.AttributeBooleanValue{Value: aws.Bool(*hostnames)},
		}); err != nil {
			return nil, err
		}
		changed = append(changed, fmt.Sprintf("dns_hostnames=%t", *hostnames))
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified VPC %s (%s)", id, strings.Join(changed, ", ")),
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
