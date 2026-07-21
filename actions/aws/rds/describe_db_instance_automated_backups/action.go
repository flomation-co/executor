// Package aws_rds_describe_db_instance_automated_backups lists RDS DB instance
// automated backups, optionally narrowed to an identifier or resource id.
package aws_rds_describe_db_instance_automated_backups

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
	Name         = "AWS RDS Describe DB Instance Automated Backups"
	Description  = "List RDS DB instance automated backups, optionally by identifier or resource id."
	Website      = "https://www.flomation.co"
	Icon         = "clock-rotate-left+magnifying-glass"
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
	{Name: "dbi_resource_id", Type: core.ConnectionTypeString, Label: "DBI Resource ID (optional)", Placeholder: "db-XXXXXXXXXXXXXXXX"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backups", Type: core.ConnectionTypeObject, Label: "Automated Backups"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeDBInstanceAutomatedBackupsInput{}
	if id := awscommon.InputString("db_instance_identifier", inputs); id != "" {
		in.DBInstanceIdentifier = aws.String(id)
	}
	if rid := awscommon.InputString("dbi_resource_id", inputs); rid != "" {
		in.DbiResourceId = aws.String(rid)
	}

	var backups []map[string]interface{}
	paginator := rds.NewDescribeDBInstanceAutomatedBackupsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.DBInstanceAutomatedBackups {
			backups = append(backups, flattenBackup(&page.DBInstanceAutomatedBackups[i]))
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d automated backup(s)", len(backups)),
		"backups":     backups,
		"count":       len(backups),
	}, nil
}

func flattenBackup(b *rdstypes.DBInstanceAutomatedBackup) map[string]interface{} {
	m := map[string]interface{}{
		"db_instance_identifier": aws.ToString(b.DBInstanceIdentifier),
		"dbi_resource_id":        aws.ToString(b.DbiResourceId),
		"status":                 aws.ToString(b.Status),
		"region":                 aws.ToString(b.Region),
		"engine":                 aws.ToString(b.Engine),
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
