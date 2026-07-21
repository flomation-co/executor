// Package aws_rds_describe_tenant_databases lists tenant databases in a
// multi-tenant RDS DB instance, optionally narrowed by instance or name.
package aws_rds_describe_tenant_databases

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
	Name         = "AWS RDS Describe Tenant Databases"
	Description  = "List tenant databases, optionally filtered by instance or name."
	Website      = "https://www.flomation.co"
	Icon         = "database+magnifying-glass"
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
	{Name: "db_instance_identifier", Type: core.ConnectionTypeString, Label: "DB Instance Identifier (optional)", Placeholder: "Leave blank to list all"},
	{Name: "tenant_db_name", Type: core.ConnectionTypeString, Label: "Tenant Database Name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tenant_databases", Type: core.ConnectionTypeObject, Label: "Tenant Databases"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeTenantDatabasesInput{}
	if v := awscommon.InputString("db_instance_identifier", inputs); v != "" {
		in.DBInstanceIdentifier = aws.String(v)
	}
	if v := awscommon.InputString("tenant_db_name", inputs); v != "" {
		in.TenantDBName = aws.String(v)
	}

	var databases []map[string]interface{}
	paginator := rds.NewDescribeTenantDatabasesPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.TenantDatabases {
			td := &page.TenantDatabases[i]
			databases = append(databases, map[string]interface{}{
				"db_instance_identifier": aws.ToString(td.DBInstanceIdentifier),
				"tenant_db_name":         aws.ToString(td.TenantDBName),
				"status":                 aws.ToString(td.Status),
				"master_username":        aws.ToString(td.MasterUsername),
			})
		}
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Found %d tenant database(s)", len(databases)),
		"tenant_databases": databases,
		"count":            len(databases),
	}, nil
}
