// Package aws_rds_modify_integration modifies an existing zero-ETL integration.
package aws_rds_modify_integration

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Modify Integration"
	Description  = "Modify a zero-ETL integration, e.g. rename it."
	Website      = "https://www.flomation.co"
	Icon         = "link+pen"
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
	{Name: "integration_identifier", Type: core.ConnectionTypeString, Label: "Integration Identifier", Placeholder: "my-zero-etl-integration", Required: true},
	{Name: "integration_name", Type: core.ConnectionTypeString, Label: "New Integration Name (optional)", Placeholder: "Leave blank to keep the current name"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "integration", Type: core.ConnectionTypeObject, Label: "Integration"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	id := awscommon.InputString("integration_identifier", inputs)
	if id == "" {
		return nil, fmt.Errorf("integration identifier is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.ModifyIntegrationInput{IntegrationIdentifier: aws.String(id)}
	if name := awscommon.InputString("integration_name", inputs); name != "" {
		in.IntegrationName = aws.String(name)
	}

	out, err := client.ModifyIntegration(ctx, in)
	if err != nil {
		return nil, err
	}

	integration := map[string]interface{}{
		"integration_name": aws.ToString(out.IntegrationName),
		"integration_arn":  aws.ToString(out.IntegrationArn),
		"source_arn":       aws.ToString(out.SourceArn),
		"target_arn":       aws.ToString(out.TargetArn),
		"status":           string(out.Status),
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Modified integration %q (status: %s)", id, string(out.Status)),
		"integration": integration,
	}, nil
}
