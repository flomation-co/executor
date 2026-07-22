// Package aws_iam_get_open_id_connect_provider retrieves an IAM OIDC identity provider.
package aws_iam_get_open_id_connect_provider

import (
	"context"
	"encoding/json"
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
	Name         = "AWS IAM Get OIDC Provider"
	Description  = "Retrieve an IAM OpenID Connect identity provider's URL, client IDs and thumbprints."
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
	{Name: "open_id_connect_provider_arn", Type: core.ConnectionTypeString, Label: "OIDC Provider ARN", Placeholder: "arn:aws:iam::<account>:oidc-provider/token.actions.githubusercontent.com", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "url", Type: core.ConnectionTypeString, Label: "Provider URL"},
	{Name: "client_ids", Type: core.ConnectionTypeString, Label: "Client IDs (JSON)"},
	{Name: "thumbprints", Type: core.ConnectionTypeString, Label: "Thumbprints (JSON)"},
	{Name: "create_date", Type: core.ConnectionTypeString, Label: "Create Date"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	arn := strings.TrimSpace(awscommon.InputString("open_id_connect_provider_arn", inputs))
	if arn == "" {
		return nil, fmt.Errorf("oidc provider arn is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	out, err := client.GetOpenIDConnectProvider(ctx, &iam.GetOpenIDConnectProviderInput{OpenIDConnectProviderArn: aws.String(arn)})
	if err != nil {
		return nil, err
	}

	clientIDs, err := json.Marshal(out.ClientIDList)
	if err != nil {
		return nil, err
	}
	thumbprints, err := json.Marshal(out.ThumbprintList)
	if err != nil {
		return nil, err
	}
	var createDate string
	if out.CreateDate != nil {
		createDate = out.CreateDate.Format(time.RFC3339)
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Retrieved OIDC provider %s", aws.ToString(out.Url)),
		"url":         aws.ToString(out.Url),
		"client_ids":  string(clientIDs),
		"thumbprints": string(thumbprints),
		"create_date": createDate,
	}, nil
}
