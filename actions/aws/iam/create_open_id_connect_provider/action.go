// Package aws_iam_create_open_id_connect_provider creates an IAM OIDC identity provider.
package aws_iam_create_open_id_connect_provider

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Create OIDC Provider"
	Description  = "Create an IAM OpenID Connect identity provider for SSO federation."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+plus"
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
	{Name: "url", Type: core.ConnectionTypeString, Label: "Provider URL", Placeholder: "https://token.actions.githubusercontent.com", Required: true},
	{Name: "client_id_list", Type: core.ConnectionTypeString, Label: "Client IDs (comma-separated)", Placeholder: "sts.amazonaws.com", Required: true},
	{Name: "thumbprint_list", Type: core.ConnectionTypeString, Label: "Thumbprints (comma-separated, optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "open_id_connect_provider_arn", Type: core.ConnectionTypeString, Label: "OIDC Provider ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	url := strings.TrimSpace(awscommon.InputString("url", inputs))
	if url == "" {
		return nil, fmt.Errorf("provider url is required")
	}
	clientIDs := splitList(awscommon.InputString("client_id_list", inputs))
	if len(clientIDs) == 0 {
		return nil, fmt.Errorf("at least one client id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.CreateOpenIDConnectProviderInput{
		Url:          aws.String(url),
		ClientIDList: clientIDs,
	}
	if thumbprints := splitList(awscommon.InputString("thumbprint_list", inputs)); len(thumbprints) > 0 {
		in.ThumbprintList = thumbprints
	}
	if conn := core.FindConnection("tags", inputs); conn != nil {
		for _, kv := range conn.KeyValuePairs() {
			k := strings.TrimSpace(kv.Key)
			if k == "" {
				continue
			}
			in.Tags = append(in.Tags, iamtypes.Tag{Key: aws.String(k), Value: aws.String(kv.Value)})
		}
	}

	out, err := client.CreateOpenIDConnectProvider(ctx, in)
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.OpenIDConnectProviderArn)
	return map[string]interface{}{
		"tool_result":                  fmt.Sprintf("Created OIDC provider %s (%s)", url, arn),
		"open_id_connect_provider_arn": arn,
	}, nil
}

func splitList(raw string) []string {
	var out []string
	for _, p := range strings.Split(raw, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
