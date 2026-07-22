// Package aws_eventbridge_remove_targets removes targets from an EventBridge
// rule.
package aws_eventbridge_remove_targets

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
	Name         = "AWS EventBridge Remove Targets"
	Description  = "Remove one or more targets from an EventBridge rule by target ID."
	Website      = "https://www.flomation.co"
	Icon         = "bolt+arrow-down"
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
	{Name: "ids", Type: core.ConnectionTypeString, Label: "Target IDs", Placeholder: "target-1, target-2", Required: true},
	{Name: "event_bus_name", Type: core.ConnectionTypeString, Label: "Event Bus Name (optional)", Placeholder: "default"},
	{Name: "force", Type: core.ConnectionTypeBoolean, Label: "Force (remove managed-rule targets)"},
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
	var ids []string
	for _, id := range strings.Split(awscommon.InputString("ids", inputs), ",") {
		if t := strings.TrimSpace(id); t != "" {
			ids = append(ids, t)
		}
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("at least one target id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.RemoveTargetsInput{
		Rule:  aws.String(rule),
		Ids:   ids,
		Force: awscommon.InputBool("force", inputs),
	}
	if bus := strings.TrimSpace(awscommon.InputString("event_bus_name", inputs)); bus != "" {
		in.EventBusName = aws.String(bus)
	}

	out, err := client.RemoveTargets(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Removed %d target(s) from rule %s (%d failed)", len(ids), rule, out.FailedEntryCount),
		"failed_entry_count": int(out.FailedEntryCount),
	}, nil
}
