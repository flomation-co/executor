// Package aws_eventbridge_put_targets adds or updates targets for an
// EventBridge rule.
package aws_eventbridge_put_targets

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/eventbridge"
	ebtypes "github.com/aws/aws-sdk-go-v2/service/eventbridge/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EventBridge Put Targets"
	Description  = "Add or update the targets invoked when an EventBridge rule fires."
	Website      = "https://www.flomation.co"
	Icon         = "bolt+link"
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
	{Name: "rule", Type: core.ConnectionTypeString, Label: "Rule Name", Placeholder: "my-rule", Required: true},
	{Name: "event_bus_name", Type: core.ConnectionTypeString, Label: "Event Bus Name (optional)", Placeholder: "default"},
	{Name: "targets", Type: core.ConnectionTypeString, Label: "Targets (JSON array)", Placeholder: `[{"Id":"1","Arn":"arn:aws:sqs:eu-west-2:123:queue"}]`, Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "failed_entry_count", Type: core.ConnectionTypeInteger, Label: "Failed Entry Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	rule := strings.TrimSpace(awscommon.InputString("rule", inputs))
	if rule == "" {
		return nil, fmt.Errorf("rule name is required")
	}
	targetsRaw := strings.TrimSpace(awscommon.InputString("targets", inputs))
	if targetsRaw == "" {
		return nil, fmt.Errorf("targets JSON array is required")
	}

	var targets []ebtypes.Target
	if err := json.Unmarshal([]byte(targetsRaw), &targets); err != nil {
		return nil, fmt.Errorf("targets must be a JSON array: %w", err)
	}
	if len(targets) == 0 {
		return nil, fmt.Errorf("at least one target is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.PutTargetsInput{
		Rule:    aws.String(rule),
		Targets: targets,
	}
	if bus := strings.TrimSpace(awscommon.InputString("event_bus_name", inputs)); bus != "" {
		in.EventBusName = aws.String(bus)
	}

	out, err := client.PutTargets(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Put %d target(s) on rule %s (%d failed)", len(targets), rule, out.FailedEntryCount),
		"failed_entry_count": int(out.FailedEntryCount),
	}, nil
}
