// Package aws_cloudwatchlogs_filter_log_events searches CloudWatch Logs across streams by pattern.
package aws_cloudwatchlogs_filter_log_events

import (
	"context"
	"encoding/json"
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
	Name         = "AWS CloudWatch Filter Log Events"
	Description  = "Search CloudWatch Logs across streams by filter pattern and time range."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+magnifying-glass"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

// maxEvents caps how many matched events we accumulate across pages.
const maxEvents = 200

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
	{Name: "filter_pattern", Type: core.ConnectionTypeString, Label: "Filter Pattern (optional)", Placeholder: "ERROR"},
	{Name: "start_time", Type: core.ConnectionTypeString, Label: "Start Time (RFC3339, optional)", Placeholder: "2026-07-22T00:00:00Z"},
	{Name: "end_time", Type: core.ConnectionTypeString, Label: "End Time (RFC3339, optional)", Placeholder: "2026-07-22T23:59:59Z"},
	{Name: "limit", Type: core.ConnectionTypeInteger, Label: "Per-page Limit (optional)", Placeholder: "100"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "events", Type: core.ConnectionTypeString, Label: "Events (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	logGroup := awscommon.InputString("log_group_name", inputs)
	if logGroup == "" {
		return nil, fmt.Errorf("log group name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cloudwatchlogs.NewFromConfig(cfg)

	in := &cloudwatchlogs.FilterLogEventsInput{
		LogGroupName: aws.String(logGroup),
	}
	if p := awscommon.InputString("filter_pattern", inputs); p != "" {
		in.FilterPattern = aws.String(p)
	}
	// Events APIs use epoch MILLISECONDS.
	if s := awscommon.InputString("start_time", inputs); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("start_time must be RFC3339 (e.g. 2026-07-22T00:00:00Z): %w", err)
		}
		in.StartTime = aws.Int64(t.UnixMilli())
	}
	if s := awscommon.InputString("end_time", inputs); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, fmt.Errorf("end_time must be RFC3339 (e.g. 2026-07-22T23:59:59Z): %w", err)
		}
		in.EndTime = aws.Int64(t.UnixMilli())
	}
	if n, ok := awscommon.InputInt("limit", inputs); ok {
		in.Limit = aws.Int32(int32(n))
	}

	type eventOut struct {
		Timestamp     int64  `json:"timestamp"`
		Message       string `json:"message"`
		LogStreamName string `json:"log_stream_name"`
		EventID       string `json:"event_id"`
	}
	events := make([]eventOut, 0, maxEvents)

	for {
		out, err := client.FilterLogEvents(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, e := range out.Events {
			events = append(events, eventOut{
				Timestamp:     aws.ToInt64(e.Timestamp),
				Message:       aws.ToString(e.Message),
				LogStreamName: aws.ToString(e.LogStreamName),
				EventID:       aws.ToString(e.EventId),
			})
		}
		if len(events) >= maxEvents || out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		in.NextToken = out.NextToken
	}

	if len(events) > maxEvents {
		events = events[:maxEvents]
	}

	eventsJSON, err := json.Marshal(events)
	if err != nil {
		return nil, fmt.Errorf("marshal events: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Matched %d log event(s) in %s", len(events), logGroup),
		"events":      string(eventsJSON),
		"count":       len(events),
	}, nil
}
