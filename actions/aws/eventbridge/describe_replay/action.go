// Package aws_eventbridge_describe_replay describes an EventBridge replay.
package aws_eventbridge_describe_replay

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EventBridge Describe Replay"
	Description  = "Retrieve details about an EventBridge replay by name."
	Website      = "https://www.flomation.co"
	Icon         = "bolt+magnifying-glass"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
	{Name: "event_source_arn", Type: core.ConnectionTypeString, Label: "Event Source ARN"},
	{Name: "event_last_replayed_time", Type: core.ConnectionTypeString, Label: "Event Last Replayed Time"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	replayName := strings.TrimSpace(awscommon.InputString("replay_name", inputs))
	if replayName == "" {
		return nil, fmt.Errorf("replay name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	out, err := client.DescribeReplay(ctx, &eventbridge.DescribeReplayInput{
		ReplayName: aws.String(replayName),
	})
	if err != nil {
		return nil, err
	}

	state := string(out.State)
	eventSourceArn := aws.ToString(out.EventSourceArn)
	lastReplayed := ""
	if out.EventLastReplayedTime != nil {
		lastReplayed = out.EventLastReplayedTime.Format("2006-01-02T15:04:05Z07:00")
	}

	return map[string]interface{}{
		"tool_result":              fmt.Sprintf("Replay %s is %s", replayName, state),
		"state":                    state,
		"event_source_arn":         eventSourceArn,
		"event_last_replayed_time": lastReplayed,
	}, nil
}
