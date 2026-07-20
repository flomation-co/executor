// Package aws_rds_restore_db_instance_from_db_snapshot restores a new RDS DB
// instance from a DB snapshot.
package aws_rds_restore_db_instance_from_db_snapshot

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
	Name         = "AWS RDS Restore Instance from Snapshot"
	Description  = "Restore a new RDS database instance from a DB snapshot."
	Website      = "https://www.flomation.co"
	Icon         = "database+box-archive"
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
	{Name: "db_instance_identifier", Type: core.ConnectionTypeString, Label: "New DB Instance Identifier", Placeholder: "restored-database", Required: true},
	{Name: "db_snapshot_identifier", Type: core.ConnectionTypeString, Label: "DB Snapshot Identifier", Placeholder: "my-snapshot", Required: true},
	{Name: "db_instance_class", Type: core.ConnectionTypeString, Label: "DB Instance Class (optional)", Placeholder: "db.t3.medium"},
	{Name: "availability_zone", Type: core.ConnectionTypeString, Label: "Availability Zone (optional)", Placeholder: "eu-west-2a"},
	{Name: "multi_az", Type: core.ConnectionTypeBoolean, Label: "Multi-AZ"},
	{Name: "publicly_accessible", Type: core.ConnectionTypeBoolean, Label: "Publicly Accessible"},
	{Name: "db_subnet_group_name", Type: core.ConnectionTypeString, Label: "DB Subnet Group Name (optional)"},
	{Name: "vpc_security_group_ids", Type: core.ConnectionTypeString, Label: "VPC Security Group IDs (optional)", Placeholder: "Comma-separated, e.g. sg-123,sg-456"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "instance", Type: core.ConnectionTypeObject, Label: "DB Instance"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("db_instance_identifier", inputs)
	if id == "" {
		return nil, fmt.Errorf("new db instance identifier is required")
	}
	snapshot := awscommon.InputString("db_snapshot_identifier", inputs)
	if snapshot == "" {
		return nil, fmt.Errorf("db snapshot identifier is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.RestoreDBInstanceFromDBSnapshotInput{
		DBInstanceIdentifier: aws.String(id),
		DBSnapshotIdentifier: aws.String(snapshot),
	}
	if v := awscommon.InputString("db_instance_class", inputs); v != "" {
		in.DBInstanceClass = aws.String(v)
	}
	if v := awscommon.InputString("availability_zone", inputs); v != "" {
		in.AvailabilityZone = aws.String(v)
	}
	if awscommon.InputBool("multi_az", inputs) {
		in.MultiAZ = aws.Bool(true)
	}
	if awscommon.InputBool("publicly_accessible", inputs) {
		in.PubliclyAccessible = aws.Bool(true)
	}
	if v := awscommon.InputString("db_subnet_group_name", inputs); v != "" {
		in.DBSubnetGroupName = aws.String(v)
	}
	if sgs := awscommon.InputStrings("vpc_security_group_ids", inputs); len(sgs) > 0 {
		in.VpcSecurityGroupIds = sgs
	}
	if tags := buildTags(inputs); len(tags) > 0 {
		in.Tags = tags
	}

	out, err := client.RestoreDBInstanceFromDBSnapshot(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Restored DB instance %q from snapshot %q (status: %s)", id, snapshot, aws.ToString(out.DBInstance.DBInstanceStatus)),
		"instance":    rdscat.SummariseInstance(out.DBInstance),
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
