// Package aws_secretsmanager_create_secret creates a new secret in AWS Secrets Manager.
package aws_secretsmanager_create_secret

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Secrets Manager Create Secret"
	Description  = "Create a new secret with an optional value, description and tags."
	Website      = "https://www.flomation.co"
	Icon         = "lock+plus"
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Secret Name", Placeholder: "prod/db/password", Required: true},
	{Name: "secret_string", Type: core.ConnectionTypeSecret, Label: "Secret Value"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
	{Name: "kms_key_id", Type: core.ConnectionTypeString, Label: "KMS Key ID (optional)"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "Secret ARN"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Secret Name"},
	{Name: "version_id", Type: core.ConnectionTypeString, Label: "Version ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	name := awscommon.InputString("name", inputs)
	if name == "" {
		return nil, fmt.Errorf("secret name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := secretsmanager.NewFromConfig(cfg)

	in := &secretsmanager.CreateSecretInput{Name: aws.String(name)}
	if v := awscommon.InputString("secret_string", inputs); v != "" {
		in.SecretString = aws.String(v)
	}
	if d := awscommon.InputString("description", inputs); d != "" {
		in.Description = aws.String(d)
	}
	if k := awscommon.InputString("kms_key_id", inputs); k != "" {
		in.KmsKeyId = aws.String(k)
	}
	if conn := core.FindConnection("tags", inputs); conn != nil {
		for _, kv := range conn.KeyValuePairs() {
			if kv.Key == "" {
				continue
			}
			in.Tags = append(in.Tags, smtypes.Tag{Key: aws.String(kv.Key), Value: aws.String(kv.Value)})
		}
	}

	out, err := client.CreateSecret(ctx, in)
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.ARN)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created secret %s (%s)", aws.ToString(out.Name), arn),
		"arn":         arn,
		"name":        aws.ToString(out.Name),
		"version_id":  aws.ToString(out.VersionId),
	}, nil
}
