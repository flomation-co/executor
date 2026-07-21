// Package aws_ec2_get_console_output retrieves and decodes an EC2 instance's
// console output.
package aws_ec2_get_console_output

import (
	"context"
	"encoding/base64"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EC2 Get Console Output"
	Description  = "Retrieve and decode an EC2 instance's console output."
	Website      = "https://www.flomation.co"
	Icon         = "server+file-lines"
	Date         = "21/07/2026"
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
	{Name: "instance_id", Type: core.ConnectionTypeString, Label: "Instance ID", Placeholder: "i-0abc123", Required: true},
	{Name: "latest", Type: core.ConnectionTypeBoolean, Label: "Latest Output", Placeholder: "Retrieve the latest console output (Nitro instances)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "output", Type: core.ConnectionTypeText, Label: "Console Output"},
	{Name: "timestamp", Type: core.ConnectionTypeString, Label: "Last Updated"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	instanceID := awscommon.InputString("instance_id", inputs)
	if instanceID == "" {
		return nil, fmt.Errorf("instance id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := ec2.NewFromConfig(cfg)

	in := &ec2.GetConsoleOutputInput{InstanceId: aws.String(instanceID)}
	if awscommon.InputBool("latest", inputs) {
		in.Latest = aws.Bool(true)
	}

	out, err := client.GetConsoleOutput(ctx, in)
	if err != nil {
		return nil, err
	}

	decoded := ""
	if out.Output != nil {
		if raw, derr := base64.StdEncoding.DecodeString(*out.Output); derr == nil {
			decoded = string(raw)
		} else {
			// Some responses may already be plain text; fall back to the raw value.
			decoded = *out.Output
		}
	}

	timestamp := ""
	if out.Timestamp != nil {
		timestamp = out.Timestamp.UTC().Format("2006-01-02T15:04:05Z")
	}

	preview := decoded
	if len(preview) > 500 {
		preview = preview[:500] + "…"
	}
	if preview == "" {
		preview = "(no console output available yet)"
	}

	return map[string]interface{}{
		"tool_result": preview,
		"output":      decoded,
		"timestamp":   timestamp,
	}, nil
}
