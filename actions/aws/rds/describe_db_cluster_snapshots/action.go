// Package aws_rds_describe_db_cluster_snapshots lists Aurora/RDS DB cluster snapshots.
package aws_rds_describe_db_cluster_snapshots

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
	Name         = "AWS RDS Describe DB Cluster Snapshots"
	Description  = "List Aurora/RDS DB cluster snapshots, optionally filtered by cluster, snapshot or type."
	Website      = "https://www.flomation.co"
	Icon         = "box-archive+magnifying-glass"
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
	{Name: "db_cluster_identifier", Type: core.ConnectionTypeString, Label: "DB Cluster Identifier (optional)", Placeholder: "Leave blank to list all"},
	{Name: "db_cluster_snapshot_identifier", Type: core.ConnectionTypeString, Label: "Snapshot Identifier (optional)"},
	{Name: "snapshot_type", Type: core.ConnectionTypeString, Label: "Snapshot Type (optional)", Options: []core.ConnectionOption{
		{Name: "Any", Value: ""},
		{Name: "Manual", Value: "manual"},
		{Name: "Automated", Value: "automated"},
		{Name: "Shared", Value: "shared"},
		{Name: "Public", Value: "public"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "snapshots", Type: core.ConnectionTypeObject, Label: "DB Cluster Snapshots"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeDBClusterSnapshotsInput{}
	if v := awscommon.InputString("db_cluster_identifier", inputs); v != "" {
		in.DBClusterIdentifier = aws.String(v)
	}
	if v := awscommon.InputString("db_cluster_snapshot_identifier", inputs); v != "" {
		in.DBClusterSnapshotIdentifier = aws.String(v)
	}
	if v := awscommon.InputString("snapshot_type", inputs); v != "" {
		in.SnapshotType = aws.String(v)
	}

	var snapshots []map[string]interface{}
	paginator := rds.NewDescribeDBClusterSnapshotsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.DBClusterSnapshots {
			snapshots = append(snapshots, rdscat.SummariseClusterSnapshot(&page.DBClusterSnapshots[i]))
		}
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d DB cluster snapshot(s)", len(snapshots)),
		"snapshots":   snapshots,
		"count":       len(snapshots),
	}, nil
}
