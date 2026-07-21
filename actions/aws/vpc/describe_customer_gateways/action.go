// Package aws_vpc_describe_customer_gateways lists customer gateways, optionally
// filtered by id or tags.
package aws_vpc_describe_customer_gateways

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
	Name         = "AWS VPC Describe Customer Gateways"
	Description  = "List customer gateways, optionally filtered by id or tags."
	Website      = "https://www.flomation.co"
	Icon         = "lock+magnifying-glass"
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
	{Name: "customer_gateway_id", Type: core.ConnectionTypeString, Label: "Customer Gateway ID (optional)", Placeholder: "Leave blank to list all"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "customer_gateways", Type: core.ConnectionTypeObject, Label: "Customer Gateways"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeCustomerGatewaysInput{
		Filters: awscommon.BuildEC2Filters(inputs, nil),
	}
	if ids := awscommon.InputStrings("customer_gateway_id", inputs); len(ids) > 0 {
		in.CustomerGatewayIds = ids
	}

	out, err := client.DescribeCustomerGateways(ctx, in)
	if err != nil {
		return nil, err
	}

	var gateways []map[string]interface{}
	for i := range out.CustomerGateways {
		g := &out.CustomerGateways[i]
		gateways = append(gateways, map[string]interface{}{
			"customer_gateway_id": aws.ToString(g.CustomerGatewayId),
			"state":               aws.ToString(g.State),
			"type":                aws.ToString(g.Type),
			"ip_address":          aws.ToString(g.IpAddress),
			"bgp_asn":             aws.ToString(g.BgpAsn),
			"device_name":         aws.ToString(g.DeviceName),
			"certificate_arn":     aws.ToString(g.CertificateArn),
		})
	}

	return map[string]interface{}{
		"tool_result":       fmt.Sprintf("Found %d customer gateway(s)", len(gateways)),
		"customer_gateways": gateways,
		"count":             len(gateways),
	}, nil
}
