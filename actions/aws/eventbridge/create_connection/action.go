// Package aws_eventbridge_create_connection creates an EventBridge connection.
package aws_eventbridge_create_connection

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
	Name         = "AWS EventBridge Create Connection"
	Description  = "Create an EventBridge connection; auth_parameters is a JSON object of auth settings."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Connection Name", Placeholder: "my-connection", Required: true},
	{Name: "authorization_type", Type: core.ConnectionTypeString, Label: "Authorization Type", Required: true, Options: []core.ConnectionOption{
		{Name: "Basic", Value: "BASIC"},
		{Name: "API Key", Value: "API_KEY"},
		{Name: "OAuth Client Credentials", Value: "OAUTH_CLIENT_CREDENTIALS"},
	}},
	{Name: "auth_parameters", Type: core.ConnectionTypeString, Label: "Auth Parameters (JSON)", Placeholder: `{"BasicAuthParameters":{"Username":"u","Password":"p"}}`, Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "connection_arn", Type: core.ConnectionTypeString, Label: "Connection ARN"},
	{Name: "connection_state", Type: core.ConnectionTypeString, Label: "Connection State"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("name", inputs))
	if name == "" {
		return nil, fmt.Errorf("connection name is required")
	}
	authType := strings.TrimSpace(awscommon.InputString("authorization_type", inputs))
	if authType == "" {
		return nil, fmt.Errorf("authorization_type is required")
	}
	authParamsRaw := strings.TrimSpace(awscommon.InputString("auth_parameters", inputs))
	if authParamsRaw == "" {
		return nil, fmt.Errorf("auth_parameters JSON is required")
	}
	var authParams ebtypes.CreateConnectionAuthRequestParameters
	if err := json.Unmarshal([]byte(authParamsRaw), &authParams); err != nil {
		return nil, fmt.Errorf("auth_parameters must be a JSON object: %w", err)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.CreateConnectionInput{
		Name:              aws.String(name),
		AuthorizationType: ebtypes.ConnectionAuthorizationType(authType),
		AuthParameters:    &authParams,
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}

	out, err := client.CreateConnection(ctx, in)
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.ConnectionArn)
	state := string(out.ConnectionState)
	return map[string]interface{}{
		"tool_result":      fmt.Sprintf("Created connection %s (%s)", name, state),
		"connection_arn":   arn,
		"connection_state": state,
	}, nil
}
