// Package aws_eventbridge_create_api_destination creates an EventBridge API destination.
package aws_eventbridge_create_api_destination

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
	Name         = "AWS EventBridge Create API Destination"
	Description  = "Create an EventBridge API destination pointing at an HTTP invocation endpoint."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "API Destination Name", Placeholder: "my-destination", Required: true},
	{Name: "connection_arn", Type: core.ConnectionTypeString, Label: "Connection ARN", Placeholder: "arn:aws:events:...:connection/my-conn", Required: true},
	{Name: "invocation_endpoint", Type: core.ConnectionTypeString, Label: "Invocation Endpoint (URL)", Placeholder: "https://api.example.com/hook", Required: true},
	{Name: "http_method", Type: core.ConnectionTypeString, Label: "HTTP Method", Required: true, Options: []core.ConnectionOption{
		{Name: "POST", Value: "POST"},
		{Name: "GET", Value: "GET"},
		{Name: "PUT", Value: "PUT"},
		{Name: "PATCH", Value: "PATCH"},
		{Name: "DELETE", Value: "DELETE"},
		{Name: "HEAD", Value: "HEAD"},
		{Name: "OPTIONS", Value: "OPTIONS"},
	}},
	{Name: "invocation_rate_limit_per_second", Type: core.ConnectionTypeInteger, Label: "Invocation Rate Limit (per second)", Placeholder: "300"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "api_destination_arn", Type: core.ConnectionTypeString, Label: "API Destination ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := strings.TrimSpace(awscommon.InputString("name", inputs))
	if name == "" {
		return nil, fmt.Errorf("API destination name is required")
	}
	connectionArn := strings.TrimSpace(awscommon.InputString("connection_arn", inputs))
	if connectionArn == "" {
		return nil, fmt.Errorf("connection_arn is required")
	}
	invocationEndpoint := strings.TrimSpace(awscommon.InputString("invocation_endpoint", inputs))
	if invocationEndpoint == "" {
		return nil, fmt.Errorf("invocation_endpoint is required")
	}
	httpMethod := strings.TrimSpace(awscommon.InputString("http_method", inputs))
	if httpMethod == "" {
		return nil, fmt.Errorf("http_method is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := eventbridge.NewFromConfig(cfg)

	in := &eventbridge.CreateApiDestinationInput{
		Name:               aws.String(name),
		ConnectionArn:      aws.String(connectionArn),
		InvocationEndpoint: aws.String(invocationEndpoint),
		HttpMethod:         ebtypes.ApiDestinationHttpMethod(httpMethod),
	}
	if rate, ok := awscommon.InputInt("invocation_rate_limit_per_second", inputs); ok {
		in.InvocationRateLimitPerSecond = aws.Int32(int32(rate))
	}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}

	out, err := client.CreateApiDestination(ctx, in)
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.ApiDestinationArn)
	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Created API destination %s (%s)", name, arn),
		"api_destination_arn": arn,
	}, nil
}
