// Package aws_rds_delete_db_instance deletes an RDS database instance.
package aws_rds_delete_db_instance

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
	Name         = "AWS RDS Delete DB Instance"
	Description  = "Delete an RDS database instance, with an optional final snapshot."
	Website      = "https://www.flomation.co"
	Icon         = "database+trash"
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
	{Name: "skip_final_snapshot", Type: core.ConnectionTypeBoolean, Label: "Skip Final Snapshot"},
	{Name: "final_snapshot_identifier", Type: core.ConnectionTypeString, Label: "Final Snapshot Identifier", Placeholder: "Required unless 'Skip Final Snapshot' is on", Visible: &core.VisibleWhen{Field: "skip_final_snapshot", Values: []string{"false"}}},
	{Name: "delete_automated_backups", Type: core.ConnectionTypeBoolean, Label: "Delete Automated Backups"},
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

	skip := awscommon.InputBool("skip_final_snapshot", inputs)
	finalSnap := awscommon.InputString("final_snapshot_identifier", inputs)
	if !skip && finalSnap == "" {
		return nil, fmt.Errorf("a final snapshot identifier is required unless 'Skip Final Snapshot' is enabled")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DeleteDBInstanceInput{
		DBInstanceIdentifier:   aws.String(id),
		SkipFinalSnapshot:      aws.Bool(skip),
		DeleteAutomatedBackups: aws.Bool(awscommon.InputBool("delete_automated_backups", inputs)),
	}
	if !skip {
		in.FinalDBSnapshotIdentifier = aws.String(finalSnap)
	}

	out, err := client.DeleteDBInstance(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Deleting DB instance %q (status: %s)", id, aws.ToString(out.DBInstance.DBInstanceStatus)),
		"instance":    rdscat.SummariseInstance(out.DBInstance),
	}, nil
}
