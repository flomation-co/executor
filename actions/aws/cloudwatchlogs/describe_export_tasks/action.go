// Package aws_cloudwatchlogs_describe_export_tasks lists CloudWatch Logs export tasks.
package aws_cloudwatchlogs_describe_export_tasks

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Describe Export Tasks"
	Description  = "List log export tasks, optionally filtered by task ID or status."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+magnifying-glass"
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
	{Name: "task_id", Type: core.ConnectionTypeString, Label: "Task ID (optional)"},
	{Name: "status_code", Type: core.ConnectionTypeString, Label: "Status Code (optional)", Options: []core.ConnectionOption{
		{Name: "Cancelled", Value: "CANCELLED"},
		{Name: "Completed", Value: "COMPLETED"},
		{Name: "Failed", Value: "FAILED"},
		{Name: "Pending", Value: "PENDING"},
		{Name: "Pending Cancel", Value: "PENDING_CANCEL"},
		{Name: "Running", Value: "RUNNING"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "export_tasks", Type: core.ConnectionTypeString, Label: "Export Tasks (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type exportTaskSummary struct {
	TaskID      string `json:"task_id"`
	Status      string `json:"status"`
	Destination string `json:"destination"`
	From        string `json:"from"`
	To          string `json:"to"`
}

func msToRFC3339(ms *int64) string {
	if ms == nil {
		return ""
	}
	return time.UnixMilli(*ms).UTC().Format(time.RFC3339)
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatchlogs.NewFromConfig(cfg)

	in := &cloudwatchlogs.DescribeExportTasksInput{}
	if taskID := awscommon.InputString("task_id", inputs); taskID != "" {
		in.TaskId = aws.String(taskID)
	}
	if status := awscommon.InputString("status_code", inputs); status != "" {
		in.StatusCode = cwlogstypes.ExportTaskStatusCode(status)
	}

	out, err := client.DescribeExportTasks(ctx, in)
	if err != nil {
		return nil, err
	}

	tasks := make([]exportTaskSummary, 0, len(out.ExportTasks))
	for _, t := range out.ExportTasks {
		s := exportTaskSummary{
			From: msToRFC3339(t.From),
			To:   msToRFC3339(t.To),
		}
		if t.TaskId != nil {
			s.TaskID = *t.TaskId
		}
		if t.Destination != nil {
			s.Destination = *t.Destination
		}
		if t.Status != nil {
			s.Status = string(t.Status.Code)
		}
		tasks = append(tasks, s)
	}

	tasksJSON, err := json.Marshal(tasks)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal export tasks: %w", err)
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d export task(s)", len(tasks)),
		"export_tasks": string(tasksJSON),
		"count":        len(tasks),
	}, nil
}
