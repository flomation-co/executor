// Package aws_rds_cancel_export_task cancels an in-progress RDS export task.
package aws_rds_cancel_export_task

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
	Name         = "AWS RDS Cancel Export Task"
	Description  = "Cancel an in-progress RDS snapshot/cluster export task."
	Website      = "https://www.flomation.co"
	Icon         = "box-archive+xmark"
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
	{Name: "export_task_identifier", Type: core.ConnectionTypeString, Label: "Export Task Identifier", Placeholder: "my-export", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "export_task", Type: core.ConnectionTypeObject, Label: "Export Task"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	taskID := awscommon.InputString("export_task_identifier", inputs)
	if taskID == "" {
		return nil, fmt.Errorf("export task identifier is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	out, err := client.CancelExportTask(ctx, &rds.CancelExportTaskInput{
		ExportTaskIdentifier: aws.String(taskID),
	})
	if err != nil {
		return nil, err
	}

	exportTask := map[string]interface{}{
		"export_task_identifier": aws.ToString(out.ExportTaskIdentifier),
		"status":                 aws.ToString(out.Status),
		"source_arn":             aws.ToString(out.SourceArn),
		"s3_bucket":              aws.ToString(out.S3Bucket),
		"percent_progress":       aws.ToInt32(out.PercentProgress),
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Cancelled export task %q (status: %s)", taskID, aws.ToString(out.Status)),
		"export_task": exportTask,
	}, nil
}
