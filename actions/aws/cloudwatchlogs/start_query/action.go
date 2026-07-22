// Package aws_cloudwatchlogs_start_query starts a CloudWatch Logs Insights query.
package aws_cloudwatchlogs_start_query

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
	Name         = "AWS CloudWatch Start Query"
	Description  = "Start a CloudWatch Logs Insights query over a log group and time range."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+bolt"
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
	{Name: "query_string", Type: core.ConnectionTypeString, Label: "Query String", Placeholder: "fields @timestamp, @message | sort @timestamp desc | limit 20", Required: true},
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time (RFC3339)", Placeholder: "2026-07-22T00:00:00Z", Required: true},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time (RFC3339)", Placeholder: "2026-07-22T23:59:59Z", Required: true},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Limit (optional)", Placeholder: "1000"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "query_id", Type: core.ConnectionTypeString, Label: "Query ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	logGroup := awscommon.InputString("log_group_name", inputs)
	if logGroup == "" {
		return nil, fmt.Errorf("log group name is required")
	}
	queryString := awscommon.InputString("query_string", inputs)
	if queryString == "" {
		return nil, fmt.Errorf("query string is required")
	}
	startStr := awscommon.InputString("start_time", inputs)
	if startStr == "" {
		return nil, fmt.Errorf("start time is required")
	}
	endStr := awscommon.InputString("end_time", inputs)
	if endStr == "" {
		return nil, fmt.Errorf("end time is required")
	}

	// StartQuery uses epoch SECONDS (unlike the events APIs which use ms).
	start, err := time.Parse(time.RFC3339, startStr)
	if err != nil {
		return nil, fmt.Errorf("start_time must be RFC3339 (e.g. 2026-07-22T00:00:00Z): %w", err)
	}
	end, err := time.Parse(time.RFC3339, endStr)
	if err != nil {
		return nil, fmt.Errorf("end_time must be RFC3339 (e.g. 2026-07-22T23:59:59Z): %w", err)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatchlogs.NewFromConfig(cfg)

	in := &cloudwatchlogs.StartQueryInput{
		LogGroupName: aws.String(logGroup),
		QueryString:  aws.String(queryString),
		StartTime:    aws.Int64(start.Unix()),
		EndTime:      aws.Int64(end.Unix()),
	}
	if n, ok := awscommon.InputInt("limit", inputs); ok {
		in.Limit = aws.Int32(int32(n))
	}

	out, err := client.StartQuery(ctx, in)
	if err != nil {
		return nil, err
	}

	queryID := aws.ToString(out.QueryId)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Started Insights query %s over %s", queryID, logGroup),
		"query_id":    queryID,
	}, nil
}
