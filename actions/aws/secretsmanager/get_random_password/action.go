// Package aws_secretsmanager_get_random_password generates a random password using AWS Secrets Manager.
package aws_secretsmanager_get_random_password

import (
	"context"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS Secrets Manager Get Random Password"
	Description  = "Generate a cryptographically random password. Returns sensitive data."
	Website      = "https://www.flomation.co"
	Icon         = "lock+bolt"
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
	{Name: "password_length", Type: core.ConnectionTypeInteger, Label: "Password Length", Placeholder: "32"},
	{Name: "exclude_characters", Type: core.ConnectionTypeString, Label: "Exclude Characters (optional)"},
	{Name: "exclude_numbers", Type: core.ConnectionTypeBoolean, Label: "Exclude Numbers"},
	{Name: "exclude_punctuation", Type: core.ConnectionTypeBoolean, Label: "Exclude Punctuation"},
	{Name: "exclude_uppercase", Type: core.ConnectionTypeBoolean, Label: "Exclude Uppercase"},
	{Name: "exclude_lowercase", Type: core.ConnectionTypeBoolean, Label: "Exclude Lowercase"},
	{Name: "include_space", Type: core.ConnectionTypeBoolean, Label: "Include Space"},
	{Name: "require_each_included_type", Type: core.ConnectionTypeBoolean, Label: "Require Each Included Type"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "random_password", Type: core.ConnectionTypeString, Label: "Random Password (sensitive)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := secretsmanager.NewFromConfig(cfg)

	in := &secretsmanager.GetRandomPasswordInput{}
	if n, ok := awscommon.InputInt("password_length", inputs); ok {
		in.PasswordLength = aws.Int64(n)
	}
	if v := awscommon.InputString("exclude_characters", inputs); v != "" {
		in.ExcludeCharacters = aws.String(v)
	}
	if awscommon.InputBool("exclude_numbers", inputs) {
		in.ExcludeNumbers = aws.Bool(true)
	}
	if awscommon.InputBool("exclude_punctuation", inputs) {
		in.ExcludePunctuation = aws.Bool(true)
	}
	if awscommon.InputBool("exclude_uppercase", inputs) {
		in.ExcludeUppercase = aws.Bool(true)
	}
	if awscommon.InputBool("exclude_lowercase", inputs) {
		in.ExcludeLowercase = aws.Bool(true)
	}
	if awscommon.InputBool("include_space", inputs) {
		in.IncludeSpace = aws.Bool(true)
	}
	if awscommon.InputBool("require_each_included_type", inputs) {
		in.RequireEachIncludedType = aws.Bool(true)
	}

	out, err := client.GetRandomPassword(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":     "Generated a random password",
		"random_password": aws.ToString(out.RandomPassword),
	}, nil
}
