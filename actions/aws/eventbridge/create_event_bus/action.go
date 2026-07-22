// Package aws_eventbridge_create_event_bus creates a custom or partner
// EventBridge event bus.
package aws_eventbridge_create_event_bus

import (
	"context"
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
	Name         = "AWS EventBridge Create Event Bus"
	Description  = "Create a custom or partner EventBridge event bus."
	Website      = "https://www.flomation.co"
	Icon         = "bolt+plus"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Event Bus Name", Placeholder: "my-event-bus", Required: true},
	{Name: "event_source_name", Type: core.ConnectionTypeString, Label: "Partner Event Source Name (optional)", Placeholder: "aws.partner/example.com/..."},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags", Placeholder: "Add a Key and Value per tag"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "event_bus_arn", Type: core.ConnectionTypeString, Label: "Event Bus ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("name", inputs))
	if name == "" {
		return nil, fmt.Errorf("event bus name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.CreateEventBusInput{Name: aws.String(name)}
	if src := strings.TrimSpace(awscommon.InputString("event_source_name", inputs)); src != "" {
		in.EventSourceName = aws.String(src)
	}
	if conn := core.FindConnection("tags", inputs); conn != nil {
		for _, kv := range conn.KeyValuePairs() {
			k := strings.TrimSpace(kv.Key)
			if k == "" {
				continue
			}
			in.Tags = append(in.Tags, ebtypes.Tag{Key: aws.String(k), Value: aws.String(strings.TrimSpace(kv.Value))})
		}
	}

	out, err := client.CreateEventBus(ctx, in)
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.EventBusArn)
	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Created event bus %s (%s)", name, arn),
		"event_bus_arn": arn,
	}, nil
}
