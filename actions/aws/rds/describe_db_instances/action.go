// Package aws_rds_describe_db_instances lists RDS DB instances, optionally
// narrowed to a single identifier or specific engines.
package aws_rds_describe_db_instances

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	rdscat "flomation.app/automate/executor/actions/aws/rds"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Describe DB Instances"
	Description  = "List RDS database instances, optionally filtered by identifier or engine."
	Website      = "https://www.flomation.co"
	Icon         = "database+magnifying-glass"
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
	{Name: "db_instance_identifier", Type: core.ConnectionTypeString, Label: "DB Instance Identifier (optional)", Placeholder: "Leave blank to list all"},
	{Name: "filter_engine", Type: core.ConnectionTypeString, Label: "Filter by Engine (optional)", Placeholder: "Comma-separated, e.g. postgres,mysql"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instances", Type: core.ConnectionTypeObject, Label: "DB Instances"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeDBInstancesInput{}
	if id := awscommon.InputString("db_instance_identifier", inputs); id != "" {
		in.DBInstanceIdentifier = aws.String(id)
	}
	if engines := awscommon.InputStrings("filter_engine", inputs); len(engines) > 0 {
		in.Filters = append(in.Filters, rdstypes.Filter{Name: aws.String("engine"), Values: engines})
	}

	var instances []map[string]interface{}
	paginator := rds.NewDescribeDBInstancesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.DBInstances {
			instances = append(instances, rdscat.SummariseInstance(&page.DBInstances[i]))
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d DB instance(s)", len(instances)),
		"instances":   instances,
		"count":       len(instances),
	}, nil
}
