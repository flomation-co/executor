// Package aws_rds_describe_db_subnet_groups lists RDS DB subnet groups,
// optionally narrowed to a single name.
package aws_rds_describe_db_subnet_groups

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Describe DB Subnet Groups"
	Description  = "List RDS DB subnet groups, optionally filtered by name."
	Website      = "https://www.flomation.co"
	Icon         = "object-group+magnifying-glass"
	Date         = "20/07/2026"
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
	{Name: "db_subnet_group_name", Type: core.ConnectionTypeString, Label: "DB Subnet Group Name (optional)", Placeholder: "Leave blank to list all"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "subnet_groups", Type: core.ConnectionTypeObject, Label: "DB Subnet Groups"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeDBSubnetGroupsInput{}
	if name := awscommon.InputString("db_subnet_group_name", inputs); name != "" {
		in.DBSubnetGroupName = aws.String(name)
	}

	var groups []map[string]interface{}
	paginator := rds.NewDescribeDBSubnetGroupsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.DBSubnetGroups {
			sg := &page.DBSubnetGroups[i]
			var subnets []string
			for j := range sg.Subnets {
				subnets = append(subnets, aws.ToString(sg.Subnets[j].SubnetIdentifier))
			}
			groups = append(groups, map[string]interface{}{
				"name":        aws.ToString(sg.DBSubnetGroupName),
				"description": aws.ToString(sg.DBSubnetGroupDescription),
				"vpc_id":      aws.ToString(sg.VpcId),
				"status":      aws.ToString(sg.SubnetGroupStatus),
				"subnet_ids":  subnets,
			})
		}
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Found %d DB subnet group(s)", len(groups)),
		"subnet_groups": groups,
		"count":         len(groups),
	}, nil
}
