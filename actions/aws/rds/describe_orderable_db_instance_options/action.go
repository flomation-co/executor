// Package aws_rds_describe_orderable_db_instance_options lists the DB instance
// configurations that can be ordered for a given engine.
package aws_rds_describe_orderable_db_instance_options

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
	Name         = "AWS RDS Describe Orderable DB Instance Options"
	Description  = "List orderable RDS DB instance options for an engine (class, version, storage)."
	Website      = "https://www.flomation.co"
	Icon         = "server+magnifying-glass"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

// maxOptions caps the total collected results, as the API can return thousands.
const maxOptions = 500

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
	{Name: "engine", Type: core.ConnectionTypeString, Label: "Engine", Required: true, Options: []core.ConnectionOption{
		{Name: "PostgreSQL", Value: "postgres"},
		{Name: "MySQL", Value: "mysql"},
		{Name: "MariaDB", Value: "mariadb"},
		{Name: "Oracle (SE2)", Value: "oracle-se2"},
		{Name: "Oracle (EE)", Value: "oracle-ee"},
		{Name: "SQL Server (Express)", Value: "sqlserver-ex"},
		{Name: "SQL Server (SE)", Value: "sqlserver-se"},
		{Name: "SQL Server (EE)", Value: "sqlserver-ee"},
		{Name: "Aurora MySQL", Value: "aurora-mysql"},
		{Name: "Aurora PostgreSQL", Value: "aurora-postgresql"},
	}},
	{Name: "engine_version", Type: core.ConnectionTypeString, Label: "Engine Version (optional)", Placeholder: "e.g. 16.3"},
	{Name: "db_instance_class", Type: core.ConnectionTypeString, Label: "DB Instance Class (optional)", Placeholder: "e.g. db.t3.micro"},
	{Name: "license_model", Type: core.ConnectionTypeString, Label: "License Model (optional)", Placeholder: "e.g. license-included, general-public-license"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "options", Type: core.ConnectionTypeObject, Label: "Orderable Options"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	engine := awscommon.InputString("engine", inputs)
	if engine == "" {
		return nil, fmt.Errorf("engine is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeOrderableDBInstanceOptionsInput{Engine: aws.String(engine)}
	if v := awscommon.InputString("engine_version", inputs); v != "" {
		in.EngineVersion = aws.String(v)
	}
	if v := awscommon.InputString("db_instance_class", inputs); v != "" {
		in.DBInstanceClass = aws.String(v)
	}
	if v := awscommon.InputString("license_model", inputs); v != "" {
		in.LicenseModel = aws.String(v)
	}

	var options []map[string]interface{}
	truncated := false
	paginator := rds.NewDescribeOrderableDBInstanceOptionsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, o := range page.OrderableDBInstanceOptions {
			if len(options) >= maxOptions {
				truncated = true
				break
			}
			options = append(options, map[string]interface{}{
				"db_instance_class":           aws.ToString(o.DBInstanceClass),
				"engine":                      aws.ToString(o.Engine),
				"engine_version":              aws.ToString(o.EngineVersion),
				"storage_type":                aws.ToString(o.StorageType),
				"multi_az_capable":            aws.ToBool(o.MultiAZCapable),
				"read_replica_capable":        aws.ToBool(o.ReadReplicaCapable),
				"supports_storage_encryption": aws.ToBool(o.SupportsStorageEncryption),
			})
		}
		if truncated {
			break
		}
	}

	summary := fmt.Sprintf("Found %d orderable option(s)", len(options))
	if truncated {
		summary = fmt.Sprintf("Returning first %d orderable option(s) (results truncated)", len(options))
	}

	return map[string]interface{}{
		"tool_result": summary,
		"options":     options,
		"count":       len(options),
	}, nil
}
