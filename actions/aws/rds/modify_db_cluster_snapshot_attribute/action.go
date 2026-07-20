// Package aws_rds_modify_db_cluster_snapshot_attribute manages sharing attributes of a manual DB cluster snapshot.
package aws_rds_modify_db_cluster_snapshot_attribute

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
	Name         = "AWS RDS Modify Cluster Snapshot Attribute"
	Description  = "Authorise or revoke AWS accounts (or 'all' for public) to copy/restore a manual DB cluster snapshot."
	Website      = "https://www.flomation.co"
	Icon         = "box-archive+pen"
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
	{Name: "db_cluster_snapshot_identifier", Type: core.ConnectionTypeString, Label: "DB Cluster Snapshot Identifier", Placeholder: "my-cluster-snapshot", Required: true},
	{Name: "attribute_name", Type: core.ConnectionTypeString, Label: "Attribute", Options: []core.ConnectionOption{
		{Name: "Restore (share copy/restore)", Value: "restore"},
	}},
	{Name: "values_to_add", Type: core.ConnectionTypeString, Label: "Values to Add (optional)", Placeholder: "Comma-separated AWS account IDs, or 'all' for public"},
	{Name: "values_to_remove", Type: core.ConnectionTypeString, Label: "Values to Remove (optional)", Placeholder: "Comma-separated AWS account IDs, or 'all'"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "attribute", Type: core.ConnectionTypeObject, Label: "Cluster Snapshot Attributes"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	snapshot := awscommon.InputString("db_cluster_snapshot_identifier", inputs)
	if snapshot == "" {
		return nil, fmt.Errorf("db cluster snapshot identifier is required")
	}
	attribute := awscommon.InputString("attribute_name", inputs)
	if attribute == "" {
		attribute = "restore"
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.ModifyDBClusterSnapshotAttributeInput{
		DBClusterSnapshotIdentifier: aws.String(snapshot),
		AttributeName:               aws.String(attribute),
	}
	if add := awscommon.InputStrings("values_to_add", inputs); len(add) > 0 {
		in.ValuesToAdd = add
	}
	if remove := awscommon.InputStrings("values_to_remove", inputs); len(remove) > 0 {
		in.ValuesToRemove = remove
	}

	out, err := client.ModifyDBClusterSnapshotAttribute(ctx, in)
	if err != nil {
		return nil, err
	}

	attr := map[string]interface{}{}
	if r := out.DBClusterSnapshotAttributesResult; r != nil {
		attr["db_cluster_snapshot_identifier"] = aws.ToString(r.DBClusterSnapshotIdentifier)
		var attrs []map[string]interface{}
		for _, a := range r.DBClusterSnapshotAttributes {
			attrs = append(attrs, map[string]interface{}{
				"attribute_name":   aws.ToString(a.AttributeName),
				"attribute_values": a.AttributeValues,
			})
		}
		attr["attributes"] = attrs
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified %q attribute of DB cluster snapshot %q", attribute, snapshot),
		"attribute":   attr,
	}, nil
}
