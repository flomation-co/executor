// Package aws_rds_start_export_task exports an RDS snapshot or cluster to Amazon S3.
package aws_rds_start_export_task

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
	Name         = "AWS RDS Start Export Task"
	Description  = "Export an RDS snapshot or cluster to Amazon S3."
	Website      = "https://www.flomation.co"
	Icon         = "box-archive+arrow-up"
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
	{Name: "source_arn", Type: core.ConnectionTypeString, Label: "Source ARN (snapshot or cluster)", Placeholder: "arn:aws:rds:...:snapshot:...", Required: true},
	{Name: "s3_bucket_name", Type: core.ConnectionTypeString, Label: "S3 Bucket Name", Placeholder: "my-export-bucket", Required: true},
	{Name: "iam_role_arn", Type: core.ConnectionTypeString, Label: "IAM Role ARN", Placeholder: "arn:aws:iam::123456789012:role/rds-s3-export", Required: true},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID", Placeholder: "arn:aws:kms:...:key/...", Required: true},
	{Name: "s3_prefix", Type: core.ConnectionTypeString, Label: "S3 Prefix (optional)"},
	{Name: "export_only", Type: core.ConnectionTypeString, Label: "Export Only (optional)", Placeholder: "Comma-separated schemas/tables, e.g. mydb.public.users"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "export_task", Type: core.ConnectionTypeObject, Label: "Export Task"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	taskID := awscommon.InputString("export_task_identifier", inputs)
	sourceARN := awscommon.InputString("source_arn", inputs)
	bucket := awscommon.InputString("s3_bucket_name", inputs)
	roleARN := awscommon.InputString("iam_role_arn", inputs)
	kmsKey := awscommon.InputString("kms_key_id", inputs)
	if taskID == "" || sourceARN == "" || bucket == "" || roleARN == "" || kmsKey == "" {
		return nil, fmt.Errorf("export task identifier, source arn, s3 bucket name, iam role arn and kms key id are all required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.StartExportTaskInput{
		ExportTaskIdentifier: aws.String(taskID),
		SourceArn:            aws.String(sourceARN),
		S3BucketName:         aws.String(bucket),
		IamRoleArn:           aws.String(roleARN),
		KmsKeyId:             aws.String(kmsKey),
	}
	if p := awscommon.InputString("s3_prefix", inputs); p != "" {
		in.S3Prefix = aws.String(p)
	}
	if only := awscommon.InputStrings("export_only", inputs); len(only) > 0 {
		in.ExportOnly = only
	}

	out, err := client.StartExportTask(ctx, in)
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
		"tool_result": fmt.Sprintf("Started export task %q (status: %s)", taskID, aws.ToString(out.Status)),
		"export_task": exportTask,
	}, nil
}
