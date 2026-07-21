// Package aws_rds_describe_export_tasks lists RDS snapshot/cluster export tasks,
// optionally narrowed by export task identifier or source ARN.
package aws_rds_describe_export_tasks

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
	Name         = "AWS RDS Describe Export Tasks"
	Description  = "List RDS snapshot/cluster export tasks, optionally by identifier or source ARN."
	Website      = "https://www.flomation.co"
	Icon         = "box-archive+magnifying-glass"
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
	{Name: "export_task_identifier", Type: core.ConnectionTypeString, Label: "Export Task Identifier (optional)", Placeholder: "Leave blank to list all"},
	{Name: "source_arn", Type: core.ConnectionTypeString, Label: "Source ARN (optional)", Placeholder: "arn:aws:rds:...:snapshot:..."},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "export_tasks", Type: core.ConnectionTypeObject, Label: "Export Tasks"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeExportTasksInput{}
	if id := awscommon.InputString("export_task_identifier", inputs); id != "" {
		in.ExportTaskIdentifier = aws.String(id)
	}
	if arn := awscommon.InputString("source_arn", inputs); arn != "" {
		in.SourceArn = aws.String(arn)
	}

	var tasks []map[string]interface{}
	paginator := rds.NewDescribeExportTasksPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for i := range page.ExportTasks {
			tasks = append(tasks, flattenExportTask(&page.ExportTasks[i]))
		}
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d export task(s)", len(tasks)),
		"export_tasks": tasks,
		"count":        len(tasks),
	}, nil
}

func flattenExportTask(t *rdstypes.ExportTask) map[string]interface{} {
	m := map[string]interface{}{
		"export_task_identifier": aws.ToString(t.ExportTaskIdentifier),
		"status":                 aws.ToString(t.Status),
		"source_arn":             aws.ToString(t.SourceArn),
		"source_type":            string(t.SourceType),
		"s3_bucket":              aws.ToString(t.S3Bucket),
		"s3_prefix":              aws.ToString(t.S3Prefix),
		"iam_role_arn":           aws.ToString(t.IamRoleArn),
		"kms_key_id":             aws.ToString(t.KmsKeyId),
		"percent_progress":       aws.ToInt32(t.PercentProgress),
		"export_only":            t.ExportOnly,
	}
	if t.SnapshotTime != nil {
		m["snapshot_time"] = t.SnapshotTime.Format(time.RFC3339)
	}
	if t.TaskStartTime != nil {
		m["task_start_time"] = t.TaskStartTime.Format(time.RFC3339)
	}
	if t.TaskEndTime != nil {
		m["task_end_time"] = t.TaskEndTime.Format(time.RFC3339)
	}
	return m
}
