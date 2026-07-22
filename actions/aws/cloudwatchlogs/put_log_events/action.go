// Package aws_cloudwatchlogs_put_log_events writes log events to a CloudWatch Logs stream.
package aws_cloudwatchlogs_put_log_events

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
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
	Name         = "AWS CloudWatch Put Log Events"
	Description  = "Write one or more log events to a CloudWatch Logs stream."
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
	{Name: "log_stream_name", Type: core.ConnectionTypeString, Label: "Log Stream Name", Placeholder: "my-stream", Required: true},
	{Name: "messages", Type: core.ConnectionTypeString, Label: "Messages", Placeholder: `A plain message, or a JSON array of {"timestamp","message"}`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "next_sequence_token", Type: core.ConnectionTypeString, Label: "Next Sequence Token"},
}

type messageEntry struct {
	Timestamp int64  `json:"timestamp"`
	Message   string `json:"message"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	logGroup := awscommon.InputString("log_group_name", inputs)
	if logGroup == "" {
		return nil, fmt.Errorf("log group name is required")
	}
	logStream := awscommon.InputString("log_stream_name", inputs)
	if logStream == "" {
		return nil, fmt.Errorf("log stream name is required")
	}
	messages := awscommon.InputString("messages", inputs)
	if strings.TrimSpace(messages) == "" {
		return nil, fmt.Errorf("messages is required")
	}

	events, err := buildLogEvents(messages)
	if err != nil {
		return nil, err
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatchlogs.NewFromConfig(cfg)

	out, err := client.PutLogEvents(ctx, &cloudwatchlogs.PutLogEventsInput{
		LogGroupName:  aws.String(logGroup),
		LogStreamName: aws.String(logStream),
		LogEvents:     events,
	})
	if err != nil {
		return nil, err
	}

	nextToken := aws.ToString(out.NextSequenceToken)
	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Put %d log event(s) to %s/%s", len(events), logGroup, logStream),
		"next_sequence_token": nextToken,
	}, nil
}

// buildLogEvents accepts either a JSON array of {timestamp, message} objects or a
// plain message string. Timestamps are epoch MILLISECONDS (the events APIs use ms).
func buildLogEvents(raw string) ([]cwlogstypes.InputLogEvent, error) {
	trimmed := strings.TrimSpace(raw)
	if strings.HasPrefix(trimmed, "[") {
		var entries []messageEntry
		if err := json.Unmarshal([]byte(trimmed), &entries); err != nil {
			return nil, fmt.Errorf("messages is not a valid JSON array of {timestamp, message}: %w", err)
		}
		if len(entries) == 0 {
			return nil, fmt.Errorf("messages array is empty")
		}
		events := make([]cwlogstypes.InputLogEvent, 0, len(entries))
		for _, e := range entries {
			ts := e.Timestamp
			if ts == 0 {
				ts = time.Now().UnixMilli()
			}
			events = append(events, cwlogstypes.InputLogEvent{
				Message:   aws.String(e.Message),
				Timestamp: aws.Int64(ts),
			})
		}
		return events, nil
	}

	return []cwlogstypes.InputLogEvent{{
		Message:   aws.String(raw),
		Timestamp: aws.Int64(time.Now().UnixMilli()),
	}}, nil
}
