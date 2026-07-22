// Package aws_secretsmanager_put_secret_value stores a new value for a secret in AWS Secrets Manager.
package aws_secretsmanager_put_secret_value

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Secrets Manager Put Secret Value"
	Description  = "Store a new encrypted value as a new version of an existing secret."
	Website      = "https://www.flomation.co"
	Icon         = "lock+pen"
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
	{Name: "secret_id", Type: core.ConnectionTypeString, Label: "Secret ID or ARN", Placeholder: "prod/db/password", Required: true},
	{Name: "secret_string", Type: core.ConnectionTypeSecret, Label: "Secret Value", Required: true},
	{Name: "version_stages", Type: core.ConnectionTypeString, Label: "Version Stages (comma-separated, optional)", Placeholder: "AWSCURRENT"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "Secret ARN"},
	{Name: "version_id", Type: core.ConnectionTypeString, Label: "Version ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	secretID := awscommon.InputString("secret_id", inputs)
	if secretID == "" {
		return nil, fmt.Errorf("secret id is required")
	}
	secretString := awscommon.InputString("secret_string", inputs)
	if secretString == "" {
		return nil, fmt.Errorf("secret value is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := secretsmanager.NewFromConfig(cfg)

	in := &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(secretID),
		SecretString: aws.String(secretString),
	}
	if v := awscommon.InputString("version_stages", inputs); v != "" {
		for _, s := range strings.Split(v, ",") {
			if s = strings.TrimSpace(s); s != "" {
				in.VersionStages = append(in.VersionStages, s)
			}
		}
	}

	out, err := client.PutSecretValue(ctx, in)
	if err != nil {
		return nil, err
	}

	arn := aws.ToString(out.ARN)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Stored new value for %s (version %s)", aws.ToString(out.Name), aws.ToString(out.VersionId)),
		"arn":         arn,
		"version_id":  aws.ToString(out.VersionId),
	}, nil
}
