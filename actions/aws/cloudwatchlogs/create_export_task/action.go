// Package aws_cloudwatchlogs_create_export_task exports CloudWatch Logs log data to an Amazon S3 bucket.
package aws_cloudwatchlogs_create_export_task

import (
	"context"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Create Export Task"
	Description  = "Export a log group's events to an S3 bucket over a time range."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+arrow-up"
	Date         = "22/07/2026"
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
	{Name: "log_group_name", Type: core.ConnectionTypeString, Label: "Log Group Name", Placeholder: "/flomation/app", Required: true},
	{Name: "destination", Type: core.ConnectionTypeString, Label: "Destination S3 Bucket", Placeholder: "my-export-bucket", Required: true},
	{Name: "destination_prefix", Type: core.ConnectionTypeString, Label: "Destination Prefix (optional)", Placeholder: "exportedlogs"},
	{Name: "from_time", Type: core.ConnectionTypeString, Label: "From (RFC3339)", Placeholder: "2026-07-01T00:00:00Z", Required: true},
	{Name: "to_time", Type: core.ConnectionTypeString, Label: "To (RFC3339)", Placeholder: "2026-07-22T00:00:00Z", Required: true},
	{Name: "task_name", Type: core.ConnectionTypeString, Label: "Task Name (optional)", Placeholder: "july-export"},
	{Name: "log_stream_name_prefix", Type: core.ConnectionTypeString, Label: "Log Stream Name Prefix (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "task_id", Type: core.ConnectionTypeString, Label: "Task ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	logGroup := awscommon.InputString("log_group_name", inputs)
	if logGroup == "" {
		return nil, fmt.Errorf("log group name is required")
	}
	destination := awscommon.InputString("destination", inputs)
	if destination == "" {
		return nil, fmt.Errorf("destination S3 bucket is required")
	}
	fromRaw := awscommon.InputString("from_time", inputs)
	if fromRaw == "" {
		return nil, fmt.Errorf("from time is required")
	}
	toRaw := awscommon.InputString("to_time", inputs)
	if toRaw == "" {
		return nil, fmt.Errorf("to time is required")
	}

	fromT, err := time.Parse(time.RFC3339, fromRaw)
	if err != nil {
		return nil, fmt.Errorf("from time must be RFC3339 (e.g. 2026-07-01T00:00:00Z): %w", err)
	}
	toT, err := time.Parse(time.RFC3339, toRaw)
	if err != nil {
		return nil, fmt.Errorf("to time must be RFC3339 (e.g. 2026-07-22T00:00:00Z): %w", err)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatchlogs.NewFromConfig(cfg)

	in := &cloudwatchlogs.CreateExportTaskInput{
		LogGroupName: aws.String(logGroup),
		Destination:  aws.String(destination),
		From:         aws.Int64(fromT.UnixMilli()),
		To:           aws.Int64(toT.UnixMilli()),
	}
	if prefix := awscommon.InputString("destination_prefix", inputs); prefix != "" {
		in.DestinationPrefix = aws.String(prefix)
	}
	if taskName := awscommon.InputString("task_name", inputs); taskName != "" {
		in.TaskName = aws.String(taskName)
	}
	if streamPrefix := awscommon.InputString("log_stream_name_prefix", inputs); streamPrefix != "" {
		in.LogStreamNamePrefix = aws.String(streamPrefix)
	}

	out, err := client.CreateExportTask(ctx, in)
	if err != nil {
		return nil, err
	}

	taskID := ""
	if out.TaskId != nil {
		taskID = *out.TaskId
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created export task %s for %s to %s", taskID, logGroup, destination),
		"task_id":     taskID,
	}, nil
}
