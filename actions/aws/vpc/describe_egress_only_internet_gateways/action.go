// Package aws_vpc_describe_egress_only_internet_gateways lists egress-only
// internet gateways.
package aws_vpc_describe_egress_only_internet_gateways

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
	Name         = "AWS VPC Describe Egress-Only Internet Gateways"
	Description  = "List egress-only internet gateways, optionally by id or tags."
	Website      = "https://www.flomation.co"
	Icon         = "globe+magnifying-glass"
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
	{Name: "egress_only_internet_gateway_id", Type: core.ConnectionTypeString, Label: "Egress-Only Internet Gateway IDs (optional)", Placeholder: "Comma-separated; blank lists all"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags", Placeholder: "Only return gateways with these tags (blank Value matches any value for that key)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "egress_only_internet_gateways", Type: core.ConnectionTypeObject, Label: "Egress-Only Internet Gateways"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeEgressOnlyInternetGatewaysInput{}
	if ids := awscommon.InputStrings("egress_only_internet_gateway_id", inputs); len(ids) > 0 {
		in.EgressOnlyInternetGatewayIds = ids
	}
	if filters := awscommon.BuildEC2Filters(inputs, nil); len(filters) > 0 {
		in.Filters = filters
	}

	var gateways []map[string]interface{}
	paginator := ec2.NewDescribeEgressOnlyInternetGatewaysPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, g := range page.EgressOnlyInternetGateways {
			state := ""
			vpc := ""
			if len(g.Attachments) > 0 {
				state = string(g.Attachments[0].State)
				vpc = aws.ToString(g.Attachments[0].VpcId)
			}
			gateways = append(gateways, map[string]interface{}{
				"egress_only_internet_gateway_id": aws.ToString(g.EgressOnlyInternetGatewayId),
				"state":                           state,
				"vpc_id":                          vpc,
			})
		}
	}

	return map[string]interface{}{
		"tool_result":                   fmt.Sprintf("Found %d egress-only internet gateway(s)", len(gateways)),
		"egress_only_internet_gateways": gateways,
		"count":                         len(gateways),
	}, nil
}
