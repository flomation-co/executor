// Package aws_rds_copy_db_cluster_snapshot copies a manual Aurora/RDS DB cluster snapshot.
package aws_rds_copy_db_cluster_snapshot

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	rdscat "flomation.app/automate/executor/actions/aws/rds"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Copy DB Cluster Snapshot"
	Description  = "Copy a manual DB cluster snapshot, optionally cross-region or re-encrypted with a new KMS key."
	Website      = "https://www.flomation.co"
	Icon         = "box-archive+copy"
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
	{Name: "source_db_cluster_snapshot_identifier", Type: core.ConnectionTypeString, Label: "Source Cluster Snapshot Identifier or ARN", Placeholder: "my-cluster-snapshot (ARN for cross-region)", Required: true},
	{Name: "target_db_cluster_snapshot_identifier", Type: core.ConnectionTypeString, Label: "Target Cluster Snapshot Identifier", Placeholder: "my-cluster-snapshot-copy", Required: true},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID (optional)", Placeholder: "key ARN, ID, alias ARN, or alias name"},
	{Name: "copy_tags", Type: core.ConnectionTypeBoolean, Label: "Copy Tags from Source"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "snapshot", Type: core.ConnectionTypeObject, Label: "DB Cluster Snapshot"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	source := awscommon.InputString("source_db_cluster_snapshot_identifier", inputs)
	target := awscommon.InputString("target_db_cluster_snapshot_identifier", inputs)
	if source == "" || target == "" {
		return nil, fmt.Errorf("both source and target db cluster snapshot identifiers are required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.CopyDBClusterSnapshotInput{
		SourceDBClusterSnapshotIdentifier: aws.String(source),
		TargetDBClusterSnapshotIdentifier: aws.String(target),
	}
	if kms := awscommon.InputString("kms_key_id", inputs); kms != "" {
		in.KmsKeyId = aws.String(kms)
	}
	if awscommon.InputBool("copy_tags", inputs) {
		in.CopyTags = aws.Bool(true)
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.CopyDBClusterSnapshot(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Copying cluster snapshot %q to %q (status: %s)", source, target, aws.ToString(out.DBClusterSnapshot.Status)),
		"snapshot":    rdscat.SummariseClusterSnapshot(out.DBClusterSnapshot),
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
