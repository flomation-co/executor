// Package aws_rds_modify_db_instance modifies settings of an RDS database instance.
package aws_rds_modify_db_instance

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	rdscat "flomation.app/automate/executor/actions/aws/rds"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Modify DB Instance"
	Description  = "Modify an RDS database instance (class, storage, version or password)."
	Website      = "https://www.flomation.co"
	Icon         = "database+pen"
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
	{Name: "db_instance_identifier", Type: core.ConnectionTypeString, Label: "DB Instance Identifier", Placeholder: "my-database", Required: true},
	{Name: "db_instance_class", Type: core.ConnectionTypeString, Label: "New Instance Class (optional)", Placeholder: "db.t3.small"},
	{Name: "allocated_storage", Type: core.ConnectionTypeInteger, Label: "New Allocated Storage (GiB, optional)"},
	{Name: "engine_version", Type: core.ConnectionTypeString, Label: "New Engine Version (optional)"},
	{Name: "master_password", Type: core.ConnectionTypeSecret, Label: "New Master Password (optional)"},
	{Name: "multi_az", Type: core.ConnectionTypeString, Label: "Multi-AZ", Options: []core.ConnectionOption{
		{Name: "No change", Value: ""},
		{Name: "Convert to Multi-AZ", Value: "true"},
		{Name: "Convert to Single-AZ", Value: "false"},
	}},
	{Name: "backup_retention_period", Type: core.ConnectionTypeInteger, Label: "New Backup Retention (days, optional)"},
	{Name: "vpc_security_group_ids", Type: core.ConnectionTypeString, Label: "New VPC Security Group IDs (optional)", Placeholder: "Comma-separated; replaces the current set"},
	{Name: "deletion_protection", Type: core.ConnectionTypeString, Label: "Deletion Protection", Options: []core.ConnectionOption{
		{Name: "No change", Value: ""},
		{Name: "Enable", Value: "true"},
		{Name: "Disable", Value: "false"},
	}},
	{Name: "auto_minor_version_upgrade", Type: core.ConnectionTypeString, Label: "Auto Minor Version Upgrade", Options: []core.ConnectionOption{
		{Name: "No change", Value: ""},
		{Name: "Enable", Value: "true"},
		{Name: "Disable", Value: "false"},
	}},
	{Name: "apply_immediately", Type: core.ConnectionTypeBoolean, Label: "Apply Immediately"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance", Type: core.ConnectionTypeObject, Label: "DB Instance"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("db_instance_identifier", inputs)
	if id == "" {
		return nil, fmt.Errorf("db instance identifier is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.ModifyDBInstanceInput{
		DBInstanceIdentifier: aws.String(id),
		ApplyImmediately:     aws.Bool(awscommon.InputBool("apply_immediately", inputs)),
	}
	changed := 0
	if v := awscommon.InputString("db_instance_class", inputs); v != "" {
		in.DBInstanceClass = aws.String(v)
		changed++
	}
	if v, ok := awscommon.InputInt("allocated_storage", inputs); ok {
		in.AllocatedStorage = aws.Int32(int32(v))
		changed++
	}
	if v := awscommon.InputString("engine_version", inputs); v != "" {
		in.EngineVersion = aws.String(v)
		changed++
	}
	if v := awscommon.InputString("master_password", inputs); v != "" {
		in.MasterUserPassword = aws.String(v)
		changed++
	}
	if v := awscommon.InputString("multi_az", inputs); v != "" {
		in.MultiAZ = aws.Bool(v == "true")
		changed++
	}
	if n, ok := awscommon.InputInt("backup_retention_period", inputs); ok {
		in.BackupRetentionPeriod = aws.Int32(int32(n))
		changed++
	}
	if ids := awscommon.InputStrings("vpc_security_group_ids", inputs); len(ids) > 0 {
		in.VpcSecurityGroupIds = ids
		changed++
	}
	if v := awscommon.InputString("deletion_protection", inputs); v != "" {
		in.DeletionProtection = aws.Bool(v == "true")
		changed++
	}
	if v := awscommon.InputString("auto_minor_version_upgrade", inputs); v != "" {
		in.AutoMinorVersionUpgrade = aws.Bool(v == "true")
		changed++
	}
	if changed == 0 {
		return nil, fmt.Errorf("provide at least one setting to modify (class, storage, engine version or password)")
	}

	out, err := client.ModifyDBInstance(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modifying DB instance %q (%d change(s), status: %s)", id, changed, aws.ToString(out.DBInstance.DBInstanceStatus)),
		"instance":    rdscat.SummariseInstance(out.DBInstance),
	}, nil
}
