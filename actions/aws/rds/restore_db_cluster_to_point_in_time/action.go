// Package aws_rds_restore_db_cluster_to_point_in_time restores a new Aurora/RDS
// DB cluster to a specific point in time from a source cluster's backups.
package aws_rds_restore_db_cluster_to_point_in_time

import (
	"context"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	rdscat "flomation.app/automate/executor/actions/aws/rds"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Restore Cluster to Point in Time"
	Description  = "Restore a new Aurora/RDS DB cluster to a point in time from source backups."
	Website      = "https://www.flomation.co"
	Icon         = "circle-nodes+clock-rotate-left"
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
	{Name: "db_cluster_identifier", Type: core.ConnectionTypeString, Label: "New DB Cluster Identifier", Placeholder: "restored-cluster", Required: true},
	{Name: "source_db_cluster_identifier", Type: core.ConnectionTypeString, Label: "Source DB Cluster Identifier", Placeholder: "my-cluster", Required: true},
	{Name: "restore_type", Type: core.ConnectionTypeString, Label: "Restore Type (optional)", Options: []core.ConnectionOption{
		{Name: "Full Copy", Value: "full-copy"},
		{Name: "Copy on Write", Value: "copy-on-write"},
	}},
	{Name: "use_latest_restorable_time", Type: core.ConnectionTypeBoolean, Label: "Use Latest Restorable Time"},
	{Name: "restore_to_time", Type: core.ConnectionTypeString, Label: "Restore to Time (optional)", Placeholder: "RFC 3339, e.g. 2026-07-20T14:30:00Z"},
	{Name: "db_subnet_group_name", Type: core.ConnectionTypeString, Label: "DB Subnet Group Name (optional)"},
	{Name: "vpc_security_group_ids", Type: core.ConnectionTypeString, Label: "VPC Security Group IDs (optional)", Placeholder: "Comma-separated, e.g. sg-123,sg-456"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "cluster", Type: core.ConnectionTypeObject, Label: "DB Cluster"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("db_cluster_identifier", inputs)
	if id == "" {
		return nil, fmt.Errorf("new db cluster identifier is required")
	}
	source := awscommon.InputString("source_db_cluster_identifier", inputs)
	if source == "" {
		return nil, fmt.Errorf("source db cluster identifier is required")
	}

	in := &rds.RestoreDBClusterToPointInTimeInput{
		DBClusterIdentifier:       aws.String(id),
		SourceDBClusterIdentifier: aws.String(source),
	}
	if v := awscommon.InputString("restore_type", inputs); v != "" {
		in.RestoreType = aws.String(v)
	}
	if awscommon.InputBool("use_latest_restorable_time", inputs) {
		in.UseLatestRestorableTime = aws.Bool(true)
	}
	if restoreTimeStr := awscommon.InputString("restore_to_time", inputs); restoreTimeStr != "" {
		t, err := time.Parse(time.RFC3339, restoreTimeStr)
		if err != nil {
			return nil, fmt.Errorf("restore to time must be RFC 3339 (e.g. 2026-07-20T14:30:00Z): %w", err)
		}
		in.RestoreToTime = aws.Time(t)
	}
	if v := awscommon.InputString("db_subnet_group_name", inputs); v != "" {
		in.DBSubnetGroupName = aws.String(v)
	}
	if sgs := awscommon.InputStrings("vpc_security_group_ids", inputs); len(sgs) > 0 {
		in.VpcSecurityGroupIds = sgs
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	out, err := client.RestoreDBClusterToPointInTime(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Restored DB cluster %q from source %q (status: %s)", id, source, aws.ToString(out.DBCluster.Status)),
		"cluster":     rdscat.SummariseCluster(out.DBCluster),
	}, nil
}
