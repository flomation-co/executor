// Package aws_rds_start_db_instance_automated_backups_replication enables
// cross-region replication of automated backups for an RDS DB instance.
package aws_rds_start_db_instance_automated_backups_replication

import (
	"context"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Start Automated Backups Replication"
	Description  = "Enable cross-region replication of automated backups for an RDS DB instance."
	Website      = "https://www.flomation.co"
	Icon         = "clock-rotate-left+play"
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
	{Name: "source_db_instance_arn", Type: core.ConnectionTypeString, Label: "Source DB Instance ARN", Placeholder: "arn:aws:rds:us-east-1:123456789012:db:my-database", Required: true},
	{Name: "backup_retention_period", Type: core.ConnectionTypeInteger, Label: "Backup Retention Period (days, optional)", Placeholder: "7"},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backup", Type: core.ConnectionTypeObject, Label: "Automated Backup"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	sourceARN := awscommon.InputString("source_db_instance_arn", inputs)
	if sourceARN == "" {
		return nil, fmt.Errorf("source db instance arn is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.StartDBInstanceAutomatedBackupsReplicationInput{
		SourceDBInstanceArn: aws.String(sourceARN),
	}
	if n, ok := awscommon.InputInt("backup_retention_period", inputs); ok {
		in.BackupRetentionPeriod = aws.Int32(int32(n))
	}
	if k := awscommon.InputString("kms_key_id", inputs); k != "" {
		in.KmsKeyId = aws.String(k)
	}

	out, err := client.StartDBInstanceAutomatedBackupsReplication(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Started automated backups replication for %q (status: %s)", sourceARN, statusOf(out.DBInstanceAutomatedBackup)),
		"backup":      flattenBackup(out.DBInstanceAutomatedBackup),
	}, nil
}

func statusOf(b *rdstypes.DBInstanceAutomatedBackup) string {
	if b == nil {
		return "unknown"
	}
	return aws.ToString(b.Status)
}

func flattenBackup(b *rdstypes.DBInstanceAutomatedBackup) map[string]interface{} {
	if b == nil {
		return nil
	}
	m := map[string]interface{}{
		"db_instance_identifier":            aws.ToString(b.DBInstanceIdentifier),
		"dbi_resource_id":                   aws.ToString(b.DbiResourceId),
		"db_instance_automated_backups_arn": aws.ToString(b.DBInstanceAutomatedBackupsArn),
		"status":                            aws.ToString(b.Status),
		"region":                            aws.ToString(b.Region),
		"engine":                            aws.ToString(b.Engine),
	}
	if b.RestoreWindow != nil {
		if b.RestoreWindow.EarliestTime != nil {
			m["earliest_restore_time"] = b.RestoreWindow.EarliestTime.Format(time.RFC3339)
		}
		if b.RestoreWindow.LatestTime != nil {
			m["latest_restore_time"] = b.RestoreWindow.LatestTime.Format(time.RFC3339)
		}
	}
	return m
}
