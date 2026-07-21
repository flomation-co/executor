// Package aws_rds_modify_tenant_database modifies a tenant database within a
// multi-tenant RDS DB instance (rename or rotate the master password).
package aws_rds_modify_tenant_database

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
	Name         = "AWS RDS Modify Tenant Database"
	Description  = "Rename a tenant database or change its master password."
	Website      = "https://www.flomation.co"
	Icon         = "database+pen"
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
	{Name: "db_instance_identifier", Type: core.ConnectionTypeString, Label: "DB Instance Identifier", Placeholder: "my-database", Required: true},
	{Name: "tenant_db_name", Type: core.ConnectionTypeString, Label: "Tenant Database Name", Placeholder: "tenant1", Required: true},
	{Name: "master_user_password", Type: core.ConnectionTypeSecret, Label: "New Master Password (optional)"},
	{Name: "new_tenant_db_name", Type: core.ConnectionTypeString, Label: "New Tenant Database Name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tenant_database", Type: core.ConnectionTypeObject, Label: "Tenant Database"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	instance := awscommon.InputString("db_instance_identifier", inputs)
	tenant := awscommon.InputString("tenant_db_name", inputs)
	if instance == "" || tenant == "" {
		return nil, fmt.Errorf("both db instance identifier and tenant database name are required")
	}

	password := awscommon.InputString("master_user_password", inputs)
	newName := awscommon.InputString("new_tenant_db_name", inputs)
	if password == "" && newName == "" {
		return nil, fmt.Errorf("at least one change (new master password or new tenant database name) is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.ModifyTenantDatabaseInput{
		DBInstanceIdentifier: aws.String(instance),
		TenantDBName:         aws.String(tenant),
	}
	if password != "" {
		in.MasterUserPassword = aws.String(password)
	}
	if newName != "" {
		in.NewTenantDBName = aws.String(newName)
	}

	out, err := client.ModifyTenantDatabase(ctx, in)
	if err != nil {
		return nil, err
	}

	td := out.TenantDatabase
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modifying tenant database %q on DB instance %q (status: %s)", tenant, instance, aws.ToString(td.Status)),
		"tenant_database": map[string]interface{}{
			"db_instance_identifier": aws.ToString(td.DBInstanceIdentifier),
			"tenant_db_name":         aws.ToString(td.TenantDBName),
			"status":                 aws.ToString(td.Status),
			"master_username":        aws.ToString(td.MasterUsername),
		},
	}, nil
}
