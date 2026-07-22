// Package aws_iam_get_saml_provider retrieves an IAM SAML identity provider.
package aws_iam_get_saml_provider

import (
	"context"
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Get SAML Provider"
	Description  = "Retrieve an IAM SAML identity provider's metadata document and validity dates."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+magnifying-glass"
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
	{Name: "saml_provider_arn", Type: core.ConnectionTypeString, Label: "SAML Provider ARN", Placeholder: "arn:aws:iam::<account>:saml-provider/MyCorpIdP", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "saml_metadata_document", Type: core.ConnectionTypeString, Label: "SAML Metadata Document (XML)"},
	{Name: "create_date", Type: core.ConnectionTypeString, Label: "Create Date"},
	{Name: "valid_until", Type: core.ConnectionTypeString, Label: "Valid Until"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	arn := strings.TrimSpace(awscommon.InputString("saml_provider_arn", inputs))
	if arn == "" {
		return nil, fmt.Errorf("saml provider arn is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	out, err := client.GetSAMLProvider(ctx, &iam.GetSAMLProviderInput{SAMLProviderArn: aws.String(arn)})
	if err != nil {
		return nil, err
	}

	var createDate, validUntil string
	if out.CreateDate != nil {
		createDate = out.CreateDate.Format(time.RFC3339)
	}
	if out.ValidUntil != nil {
		validUntil = out.ValidUntil.Format(time.RFC3339)
	}

	return map[string]interface{}{
		"tool_result":            fmt.Sprintf("Retrieved SAML provider %s", arn),
		"saml_metadata_document": aws.ToString(out.SAMLMetadataDocument),
		"create_date":            createDate,
		"valid_until":            validUntil,
	}, nil
}
