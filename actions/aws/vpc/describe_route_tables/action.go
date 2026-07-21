// Package aws_vpc_describe_route_tables lists route tables, optionally filtered.
package aws_vpc_describe_route_tables

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS VPC Describe Route Tables"
	Description  = "List route tables, optionally filtered by id, VPC id or tags."
	Website      = "https://www.flomation.co"
	Icon         = "route+magnifying-glass"
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
	{Name: "route_table_id", Type: core.ConnectionTypeString, Label: "Route Table ID (optional)", Placeholder: "Leave blank to list all"},
	{Name: "vpc_id", Type: core.ConnectionTypeString, Label: "VPC ID (optional)", Placeholder: "Filter by VPC"},
	{Name: "filter_tags", Type: core.ConnectionTypeKeyValueArray, Label: "Filter by Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "route_tables", Type: core.ConnectionTypeObject, Label: "Route Tables"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.DescribeRouteTablesInput{
		Filters: awscommon.BuildEC2Filters(inputs, []awscommon.FilterSpec{
			{Input: "route_table_id", Filter: "route-table-id"},
			{Input: "vpc_id", Filter: "vpc-id"},
		}),
	}

	var tables []map[string]interface{}
	paginator := ec2.NewDescribeRouteTablesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.RouteTables {
			r := &page.RouteTables[i]
			tables = append(tables, map[string]interface{}{
				"route_table_id":    aws.ToString(r.RouteTableId),
				"vpc_id":            aws.ToString(r.VpcId),
				"owner_id":          aws.ToString(r.OwnerId),
				"route_count":       len(r.Routes),
				"association_count": len(r.Associations),
				"name":              tagName(r.Tags),
			})
		}
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d route table(s)", len(tables)),
		"route_tables": tables,
		"count":        len(tables),
	}, nil
}

func tagName(tags []ec2types.Tag) string {
	for _, t := range tags {
		if aws.ToString(t.Key) == "Name" {
			return aws.ToString(t.Value)
		}
	}
	return ""
}
