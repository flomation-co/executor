// Package aws_eventbridge_put_events publishes a custom event to an
// EventBridge event bus.
package aws_eventbridge_put_events

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
	Name         = "AWS EventBridge Put Events"
	Description  = "Publish a custom event to an EventBridge event bus."
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
	{Name: "source", Type: core.ConnectionTypeString, Label: "Source", Placeholder: "com.mycompany.myapp", Required: true},
	{Name: "detail_type", Type: core.ConnectionTypeString, Label: "Detail Type", Placeholder: "Order Placed", Required: true},
	{Name: "detail", Type: core.ConnectionTypeString, Label: "Detail (JSON)", Placeholder: `{"orderId":"123"}`, Required: true},
	{Name: "event_bus_name", Type: core.ConnectionTypeString, Label: "Event Bus Name (optional)", Placeholder: "default"},
	{Name: "resources", Type: core.ConnectionTypeString, Label: "Resources (comma-separated ARNs)", Placeholder: "arn:aws:...,arn:aws:..."},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "event_id", Type: core.ConnectionTypeString, Label: "Event ID"},
	{Name: "failed_entry_count", Type: core.ConnectionTypeInteger, Label: "Failed Entry Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	source := strings.TrimSpace(awscommon.InputString("source", inputs))
	if source == "" {
		return nil, fmt.Errorf("source is required")
	}
	detailType := strings.TrimSpace(awscommon.InputString("detail_type", inputs))
	if detailType == "" {
		return nil, fmt.Errorf("detail_type is required")
	}
	detail := strings.TrimSpace(awscommon.InputString("detail", inputs))
	if detail == "" {
		return nil, fmt.Errorf("detail JSON is required")
	}

	entry := ebtypes.PutEventsRequestEntry{
		Source:     aws.String(source),
		DetailType: aws.String(detailType),
		Detail:     aws.String(detail),
	}
	if bus := strings.TrimSpace(awscommon.InputString("event_bus_name", inputs)); bus != "" {
		entry.EventBusName = aws.String(bus)
	}
	if resourcesRaw := strings.TrimSpace(awscommon.InputString("resources", inputs)); resourcesRaw != "" {
		var resources []string
		for _, r := range strings.Split(resourcesRaw, ",") {
			if r = strings.TrimSpace(r); r != "" {
				resources = append(resources, r)
			}
		}
		entry.Resources = resources
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	out, err := client.PutEvents(ctx, &eventbridge.PutEventsInput{
		Entries: []ebtypes.PutEventsRequestEntry{entry},
	})
	if err != nil {
		return nil, err
	}

	eventID := ""
	if len(out.Entries) > 0 {
		eventID = aws.ToString(out.Entries[0].EventId)
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Published event from %s (%d failed)", source, out.FailedEntryCount),
		"event_id":           eventID,
		"failed_entry_count": int(out.FailedEntryCount),
	}, nil
}
