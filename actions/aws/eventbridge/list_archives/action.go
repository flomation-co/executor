// Package aws_eventbridge_list_archives lists EventBridge archives.
package aws_eventbridge_list_archives

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
	Name         = "AWS EventBridge List Archives"
	Description  = "List EventBridge archives, optionally filtered by prefix, source or state."
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
	{Name: "name_prefix", Type: core.ConnectionTypeString, Label: "Name Prefix", Placeholder: "Optional"},
	{Name: "event_source_arn", Type: core.ConnectionTypeString, Label: "Event Source ARN", Placeholder: "Optional event bus ARN"},
	{Name: "state", Type: core.ConnectionTypeString, Label: "State", Placeholder: "Optional (e.g. ENABLED)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "archives", Type: core.ConnectionTypeString, Label: "Archives (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.ListArchivesInput{}
	if prefix := strings.TrimSpace(awscommon.InputString("name_prefix", inputs)); prefix != "" {
		in.NamePrefix = aws.String(prefix)
	}
	if src := strings.TrimSpace(awscommon.InputString("event_source_arn", inputs)); src != "" {
		in.EventSourceArn = aws.String(src)
	}
	if state := strings.TrimSpace(awscommon.InputString("state", inputs)); state != "" {
		in.State = ebtypes.ArchiveState(state)
	}

	out, err := client.ListArchives(ctx, in)
	if err != nil {
		return nil, err
	}

	type archiveInfo struct {
		Name       string `json:"name"`
		Arn        string `json:"arn"`
		State      string `json:"state"`
		EventCount int64  `json:"event_count"`
	}
	archives := make([]archiveInfo, 0, len(out.Archives))
	for _, a := range out.Archives {
		archives = append(archives, archiveInfo{
			Name:       aws.ToString(a.ArchiveName),
			Arn:        aws.ToString(a.EventSourceArn),
			State:      string(a.State),
			EventCount: a.EventCount,
		})
	}

	encoded, err := json.Marshal(archives)
	if err != nil {
		return nil, fmt.Errorf("could not encode archives: %w", err)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Found %d archive(s)", len(archives)),
		"archives":    string(encoded),
		"count":       len(archives),
	}, nil
}
