// Package aws_eventbridge_list_rules lists EventBridge rules.
package aws_eventbridge_list_rules

import (
	"context"
	"encoding/json"
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
	Name         = "AWS EventBridge List Rules"
	Description  = "List EventBridge rules, optionally filtered by a name prefix."
	Website      = "https://www.flomation.co"
	Icon         = "bolt+list"
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
	{Name: "name_prefix", Type: core.ConnectionTypeString, Label: "Name Prefix (optional)", Placeholder: "my-"},
	{Name: "event_bus_name", Type: core.ConnectionTypeString, Label: "Event Bus Name (optional)", Placeholder: "default"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "rules", Type: core.ConnectionTypeString, Label: "Rules (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Rule Count"},
}

type ruleSummary struct {
	Name               string `json:"name"`
	Arn                string `json:"arn"`
	State              string `json:"state"`
	ScheduleExpression string `json:"schedule_expression"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.ListRulesInput{}
	if prefix := strings.TrimSpace(awscommon.InputString("name_prefix", inputs)); prefix != "" {
		in.NamePrefix = aws.String(prefix)
	}
	if bus := strings.TrimSpace(awscommon.InputString("event_bus_name", inputs)); bus != "" {
		in.EventBusName = aws.String(bus)
	}

	summaries := make([]ruleSummary, 0)
	for {
		out, err := client.ListRules(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, r := range out.Rules {
			summaries = append(summaries, ruleSummary{
				Name:               aws.ToString(r.Name),
				Arn:                aws.ToString(r.Arn),
				State:              string(r.State),
				ScheduleExpression: aws.ToString(r.ScheduleExpression),
			})
		}
		if out.NextToken == nil || aws.ToString(out.NextToken) == "" {
			break
		}
		in.NextToken = out.NextToken
	}

	encoded, err := json.Marshal(summaries)
	if err != nil {
		return nil, fmt.Errorf("failed to encode rules: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d rule(s)", len(summaries)),
		"rules":       string(encoded),
		"count":       len(summaries),
	}, nil
}
