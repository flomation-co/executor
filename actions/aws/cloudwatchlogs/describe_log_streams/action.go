// Package aws_cloudwatchlogs_describe_log_streams lists a log group's streams.
package aws_cloudwatchlogs_describe_log_streams

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	cwlogs "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
	cwlogstypes "github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS CloudWatch Logs Describe Log Streams"
	Description  = "List the log streams within a CloudWatch Logs log group."
	Website      = "https://www.flomation.co"
	Icon         = "file-lines+list"
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
	{Name: "log_group_name", Type: core.ConnectionTypeString, Label: "Log Group Name", Placeholder: "/flomation/my-app", Required: true},
	{Name: "log_stream_name_prefix", Type: core.ConnectionTypeString, Label: "Log Stream Name Prefix (optional)", Placeholder: "2026/07"},
	{Name: "order_by", Type: core.ConnectionTypeString, Label: "Order By (optional)", Options: []core.ConnectionOption{
		{Name: "Log Stream Name", Value: "LogStreamName"},
		{Name: "Last Event Time", Value: "LastEventTime"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "log_streams", Type: core.ConnectionTypeString, Label: "Log Streams (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	group := awscommon.InputString("log_group_name", inputs)
	if group == "" {
		return nil, fmt.Errorf("log group name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := cwlogs.NewFromConfig(cfg)

	in := &cwlogs.DescribeLogStreamsInput{LogGroupName: aws.String(group)}
	if prefix := awscommon.InputString("log_stream_name_prefix", inputs); prefix != "" {
		in.LogStreamNamePrefix = aws.String(prefix)
	}
	if ob := awscommon.InputString("order_by", inputs); ob != "" {
		in.OrderBy = cwlogstypes.OrderBy(ob)
		// The prefix filter is incompatible with ordering by event time.
		if in.OrderBy == cwlogstypes.OrderByLastEventTime {
			in.LogStreamNamePrefix = nil
		}
	}

	type logStreamInfo struct {
		Name        string `json:"name"`
		LastEvent   string `json:"last_event"`
		StoredBytes int64  `json:"stored_bytes"`
	}
	var streams []logStreamInfo

	paginator := cwlogs.NewDescribeLogStreamsPaginator(client, in)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, s := range page.LogStreams {
			lastEvent := ""
			if s.LastEventTimestamp != nil {
				lastEvent = time.UnixMilli(*s.LastEventTimestamp).UTC().Format(time.RFC3339)
			}
			streams = append(streams, logStreamInfo{
				Name:        aws.ToString(s.LogStreamName),
				LastEvent:   lastEvent,
				StoredBytes: aws.ToInt64(s.StoredBytes),
			})
		}
	}

	streamsJSON := "[]"
	if b, mErr := json.Marshal(streams); mErr == nil {
		streamsJSON = string(b)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d log stream(s) in %s", len(streams), group),
		"log_streams": streamsJSON,
		"count":       len(streams),
	}, nil
}
