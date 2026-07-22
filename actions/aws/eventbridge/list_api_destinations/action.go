// Package aws_eventbridge_list_api_destinations lists EventBridge API destinations.
package aws_eventbridge_list_api_destinations

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
	Name         = "AWS EventBridge List API Destinations"
	Description  = "List EventBridge API destinations, optionally filtered by name prefix."
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "api_destinations", Type: core.ConnectionTypeString, Label: "API Destinations (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.ListApiDestinationsInput{}
	if p := strings.TrimSpace(awscommon.InputString("name_prefix", inputs)); p != "" {
		in.NamePrefix = aws.String(p)
	}

	out, err := client.ListApiDestinations(ctx, in)
	if err != nil {
		return nil, err
	}

	type dest struct {
		Name     string `json:"name"`
		Arn      string `json:"arn"`
		State    string `json:"state"`
		Endpoint string `json:"endpoint"`
	}
	dests := make([]dest, 0, len(out.ApiDestinations))
	for _, d := range out.ApiDestinations {
		dests = append(dests, dest{
			Name:     aws.ToString(d.Name),
			Arn:      aws.ToString(d.ApiDestinationArn),
			State:    string(d.ApiDestinationState),
			Endpoint: aws.ToString(d.InvocationEndpoint),
		})
	}

	encoded, err := json.Marshal(dests)
	if err != nil {
		return nil, fmt.Errorf("failed to encode API destinations: %w", err)
	}

	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Found %d API destination(s)", len(dests)),
		"api_destinations": string(encoded),
		"count":            len(dests),
	}, nil
}
