// Package aws_eventbridge_start_replay starts an EventBridge archive replay.
package aws_eventbridge_start_replay

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EventBridge Start Replay"
	Description  = "Replay archived events into an event bus over a time window."
	Website      = "https://www.flomation.co"
	Icon         = "bolt+arrow-up"
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
	{Name: "replay_name", Type: core.ConnectionTypeString, Label: "Replay Name", Placeholder: "my-replay", Required: true},
	{Name: "event_source_arn", Type: core.ConnectionTypeString, Label: "Archive ARN", Placeholder: "arn:aws:events:eu-west-2:123456789012:archive/my-archive", Required: true},
	{Name: "event_start_time", Type: core.ConnectionTypeString, Label: "Event Start Time (RFC3339)", Placeholder: "2026-07-01T00:00:00Z", Required: true},
	{Name: "event_end_time", Type: core.ConnectionTypeString, Label: "Event End Time (RFC3339)", Placeholder: "2026-07-02T00:00:00Z", Required: true},
	{Name: "destination_arn", Type: core.ConnectionTypeString, Label: "Destination Event Bus ARN", Placeholder: "arn:aws:events:eu-west-2:123456789012:event-bus/default", Required: true},
	{Name: "filter_arns", Type: core.ConnectionTypeString, Label: "Filter Rule ARNs (comma-separated)", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "replay_arn", Type: core.ConnectionTypeString, Label: "Replay ARN"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	replayName := strings.TrimSpace(awscommon.InputString("replay_name", inputs))
	if replayName == "" {
		return nil, fmt.Errorf("replay name is required")
	}
	eventSourceArn := strings.TrimSpace(awscommon.InputString("event_source_arn", inputs))
	if eventSourceArn == "" {
		return nil, fmt.Errorf("event source ARN (archive ARN) is required")
	}
	destinationArn := strings.TrimSpace(awscommon.InputString("destination_arn", inputs))
	if destinationArn == "" {
		return nil, fmt.Errorf("destination ARN (event bus ARN) is required")
	}

	startRaw := strings.TrimSpace(awscommon.InputString("event_start_time", inputs))
	if startRaw == "" {
		return nil, fmt.Errorf("event start time is required")
	}
	startTime, err := time.Parse(time.RFC3339, startRaw)
	if err != nil {
		return nil, fmt.Errorf("event_start_time must be RFC3339 (e.g. 2026-07-01T00:00:00Z): %w", err)
	}
	endRaw := strings.TrimSpace(awscommon.InputString("event_end_time", inputs))
	if endRaw == "" {
		return nil, fmt.Errorf("event end time is required")
	}
	endTime, err := time.Parse(time.RFC3339, endRaw)
	if err != nil {
		return nil, fmt.Errorf("event_end_time must be RFC3339 (e.g. 2026-07-02T00:00:00Z): %w", err)
	}

	destination := &ebtypes.ReplayDestination{Arn: aws.String(destinationArn)}
	if raw := strings.TrimSpace(awscommon.InputString("filter_arns", inputs)); raw != "" {
		var filters []string
		for _, part := range strings.Split(raw, ",") {
			if p := strings.TrimSpace(part); p != "" {
				filters = append(filters, p)
			}
		}
		if len(filters) > 0 {
			destination.FilterArns = filters
		}
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	out, err := client.StartReplay(ctx, &eventbridge.StartReplayInput{
		ReplayName:     aws.String(replayName),
		EventSourceArn: aws.String(eventSourceArn),
		EventStartTime: aws.Time(startTime),
		EventEndTime:   aws.Time(endTime),
		Destination:    destination,
	})
	if err != nil {
		return nil, err
	}

	replayArn := aws.ToString(out.ReplayArn)
	state := string(out.State)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Started replay %s (%s), state %s", replayName, replayArn, state),
		"replay_arn":  replayArn,
		"state":       state,
	}, nil
}
