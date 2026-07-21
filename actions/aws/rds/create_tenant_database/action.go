// Package aws_rds_create_tenant_database creates a tenant database within a
// multi-tenant RDS DB instance (Oracle / SQL Server).
package aws_rds_create_tenant_database

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Create Tenant Database"
	Description  = "Create a tenant database in a multi-tenant RDS DB instance."
	Website      = "https://www.flomation.co"
	Icon         = "database+key"
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
	{Name: "master_username", Type: core.ConnectionTypeString, Label: "Master Username", Placeholder: "admin", Required: true},
	{Name: "master_password", Type: core.ConnectionTypeSecret, Label: "Master Password", Required: true},
	{Name: "character_set_name", Type: core.ConnectionTypeString, Label: "Character Set Name (optional)", Placeholder: "AL32UTF8"},
	{Name: "nchar_character_set_name", Type: core.ConnectionTypeString, Label: "NCHAR Character Set Name (optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "tenant_database", Type: core.ConnectionTypeObject, Label: "Tenant Database"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	instance := awscommon.InputString("db_instance_identifier", inputs)
	tenant := awscommon.InputString("tenant_db_name", inputs)
	username := awscommon.InputString("master_username", inputs)
	password := awscommon.InputString("master_password", inputs)
	if instance == "" || tenant == "" || username == "" || password == "" {
		return nil, fmt.Errorf("db instance identifier, tenant database name, master username and master password are all required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CreateTenantDatabaseInput{
		DBInstanceIdentifier: aws.String(instance),
		TenantDBName:         aws.String(tenant),
		MasterUsername:       aws.String(username),
		MasterUserPassword:   aws.String(password),
	}
	if v := awscommon.InputString("character_set_name", inputs); v != "" {
		in.CharacterSetName = aws.String(v)
	}
	if v := awscommon.InputString("nchar_character_set_name", inputs); v != "" {
		in.NcharCharacterSetName = aws.String(v)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CreateTenantDatabase(ctx, in)
	if err != nil {
		return nil, err
	}

	td := out.TenantDatabase
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Creating tenant database %q on DB instance %q (status: %s)", tenant, instance, aws.ToString(td.Status)),
		"tenant_database": map[string]interface{}{
			"db_instance_identifier": aws.ToString(td.DBInstanceIdentifier),
			"tenant_db_name":         aws.ToString(td.TenantDBName),
			"status":                 aws.ToString(td.Status),
			"master_username":        aws.ToString(td.MasterUsername),
		},
	}, nil
}

func buildTags(inputs []*core.Connection) []rdstypes.Tag {
	conn := core.FindConnection("tags", inputs)
	if conn == nil {
		return nil
	}
	var tags []rdstypes.Tag
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		tags = append(tags, rdstypes.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
	}
	return tags
}
