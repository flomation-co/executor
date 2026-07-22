// Package aws_iam_update_account_password_policy updates the account password policy.
package aws_iam_update_account_password_policy

import (
	"context"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Update Account Password Policy"
	Description  = "Update the password policy for the AWS account."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+pen"
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
	{Name: "minimum_password_length", Type: core.ConnectionTypeInteger, Label: "Minimum Password Length (optional)", Placeholder: "8"},
	{Name: "require_symbols", Type: core.ConnectionTypeBoolean, Label: "Require Symbols"},
	{Name: "require_numbers", Type: core.ConnectionTypeBoolean, Label: "Require Numbers"},
	{Name: "require_uppercase_characters", Type: core.ConnectionTypeBoolean, Label: "Require Uppercase Characters"},
	{Name: "require_lowercase_characters", Type: core.ConnectionTypeBoolean, Label: "Require Lowercase Characters"},
	{Name: "allow_users_to_change_password", Type: core.ConnectionTypeBoolean, Label: "Allow Users To Change Password"},
	{Name: "max_password_age", Type: core.ConnectionTypeInteger, Label: "Max Password Age (days, optional)", Placeholder: "90"},
	{Name: "password_reuse_prevention", Type: core.ConnectionTypeInteger, Label: "Password Reuse Prevention (optional)", Placeholder: "5"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.UpdateAccountPasswordPolicyInput{}
	if n, ok := awscommon.InputInt("minimum_password_length", inputs); ok {
		in.MinimumPasswordLength = aws.Int32(int32(n))
	}
	if n, ok := awscommon.InputInt("max_password_age", inputs); ok {
		in.MaxPasswordAge = aws.Int32(int32(n))
	}
	if n, ok := awscommon.InputInt("password_reuse_prevention", inputs); ok {
		in.PasswordReusePrevention = aws.Int32(int32(n))
	}
	// The SDK models these booleans as plain (non-pointer) bools, so a false value
	// is always sent. We only carry across a value that the user actually set.
	if core.FindConnection("require_symbols", inputs) != nil {
		in.RequireSymbols = awscommon.InputBool("require_symbols", inputs)
	}
	if core.FindConnection("require_numbers", inputs) != nil {
		in.RequireNumbers = awscommon.InputBool("require_numbers", inputs)
	}
	if core.FindConnection("require_uppercase_characters", inputs) != nil {
		in.RequireUppercaseCharacters = awscommon.InputBool("require_uppercase_characters", inputs)
	}
	if core.FindConnection("require_lowercase_characters", inputs) != nil {
		in.RequireLowercaseCharacters = awscommon.InputBool("require_lowercase_characters", inputs)
	}
	if core.FindConnection("allow_users_to_change_password", inputs) != nil {
		in.AllowUsersToChangePassword = awscommon.InputBool("allow_users_to_change_password", inputs)
	}

	_, err = client.UpdateAccountPasswordPolicy(ctx, in)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": "Updated account password policy",
	}, nil
}
