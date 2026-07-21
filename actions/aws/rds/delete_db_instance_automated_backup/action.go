// Package aws_rds_delete_db_instance_automated_backup deletes an RDS DB instance
// automated backup (a retained backup after the source instance is deleted).
package aws_rds_delete_db_instance_automated_backup

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
	Name         = "AWS RDS Delete DB Instance Automated Backup"
	Description  = "Delete a retained RDS DB instance automated backup by resource id or ARN."
	Website      = "https://www.flomation.co"
	Icon         = "clock-rotate-left+trash"
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
	{Name: "dbi_resource_id", Type: core.ConnectionTypeString, Label: "DBI Resource ID (optional)", Placeholder: "db-XXXXXXXXXXXXXXXX"},
	{Name: "db_instance_automated_backups_arn", Type: core.ConnectionTypeString, Label: "Automated Backups ARN (optional)", Placeholder: "arn:aws:rds:...:auto-backup:..."},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backup", Type: core.ConnectionTypeObject, Label: "Automated Backup"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	resourceID := awscommon.InputString("dbi_resource_id", inputs)
	arn := awscommon.InputString("db_instance_automated_backups_arn", inputs)
	if resourceID == "" && arn == "" {
		return nil, fmt.Errorf("either dbi resource id or automated backups arn is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DeleteDBInstanceAutomatedBackupInput{}
	if resourceID != "" {
		in.DbiResourceId = aws.String(resourceID)
	}
	if arn != "" {
		in.DBInstanceAutomatedBackupsArn = aws.String(arn)
	}

	out, err := client.DeleteDBInstanceAutomatedBackup(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleted automated backup (status: %s)", statusOf(out.DBInstanceAutomatedBackup)),
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
