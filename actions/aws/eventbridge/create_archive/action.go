// Package aws_eventbridge_create_archive creates an EventBridge archive.
package aws_eventbridge_create_archive

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
	Name         = "AWS EventBridge Create Archive"
	Description  = "Create an EventBridge archive that captures events from an event bus."
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
	{Name: "archive_name", Type: core.ConnectionTypeString, Label: "Archive Name", Placeholder: "my-archive", Required: true},
	{Name: "event_source_arn", Type: core.ConnectionTypeString, Label: "Event Bus ARN", Placeholder: "arn:aws:events:eu-west-2:123456789012:event-bus/default", Required: true},
	{Name: "event_pattern", Type: core.ConnectionTypeString, Label: "Event Pattern (JSON)", Placeholder: `{"source":["aws.ec2"]}`},
	{Name: "retention_days", Type: core.ConnectionTypeInteger, Label: "Retention Days (0 = indefinite)", Placeholder: "0"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "archive_arn", Type: core.ConnectionTypeString, Label: "Archive ARN"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	archiveName := strings.TrimSpace(awscommon.InputString("archive_name", inputs))
	if archiveName == "" {
		return nil, fmt.Errorf("archive name is required")
	}
	eventSourceArn := strings.TrimSpace(awscommon.InputString("event_source_arn", inputs))
	if eventSourceArn == "" {
		return nil, fmt.Errorf("event source ARN (event bus ARN) is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.CreateArchiveInput{
		ArchiveName:    aws.String(archiveName),
		EventSourceArn: aws.String(eventSourceArn),
	}
	if pattern := strings.TrimSpace(awscommon.InputString("event_pattern", inputs)); pattern != "" {
		in.EventPattern = aws.String(pattern)
	}
	if days, ok := awscommon.InputInt("retention_days", inputs); ok {
		in.RetentionDays = aws.Int32(int32(days))
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}

	out, err := client.CreateArchive(ctx, in)
	if err != nil {
		return nil, err
	}

	archiveArn := aws.ToString(out.ArchiveArn)
	state := string(out.State)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created archive %s (%s), state %s", archiveName, archiveArn, state),
		"archive_arn": archiveArn,
		"state":       state,
	}, nil
}
