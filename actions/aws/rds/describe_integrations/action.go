// Package aws_rds_describe_integrations lists zero-ETL integrations, optionally
// narrowed to a single integration identifier.
package aws_rds_describe_integrations

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/rds"
	rdstypes "github.com/aws/aws-sdk-go-v2/service/rds/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS RDS Describe Integrations"
	Description  = "List zero-ETL integrations, optionally filtered by identifier."
	Website      = "https://www.flomation.co"
	Icon         = "link+magnifying-glass"
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
	{Name: "integration_identifier", Type: core.ConnectionTypeString, Label: "Integration Identifier (optional)", Placeholder: "Leave blank to list all"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "integrations", Type: core.ConnectionTypeObject, Label: "Integrations"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := rds.NewFromConfig(cfg)

	in := &rds.DescribeIntegrationsInput{}
	if id := awscommon.InputString("integration_identifier", inputs); id != "" {
		in.IntegrationIdentifier = aws.String(id)
	}

	var integrations []map[string]interface{}
	for {
		page, err := client.DescribeIntegrations(ctx, in)
		if err != nil {
			return nil, err
		}
		for i := range page.Integrations {
			integrations = append(integrations, flattenIntegration(&page.Integrations[i]))
		}
		if page.Marker == nil || aws.ToString(page.Marker) == "" {
			break
		}
		in.Marker = page.Marker
	}

	return map[string]interface{}{
		"tool_result":  fmt.Sprintf("Found %d integration(s)", len(integrations)),
		"integrations": integrations,
		"count":        len(integrations),
	}, nil
}

func flattenIntegration(in *rdstypes.Integration) map[string]interface{} {
	return map[string]interface{}{
		"integration_name": aws.ToString(in.IntegrationName),
		"integration_arn":  aws.ToString(in.IntegrationArn),
		"source_arn":       aws.ToString(in.SourceArn),
		"target_arn":       aws.ToString(in.TargetArn),
		"status":           string(in.Status),
	}
}
