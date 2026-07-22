// Package aws_iam_create_login_profile creates a console login profile
// (password) for an IAM user.
package aws_iam_create_login_profile

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
	Name         = "AWS IAM Create Login Profile"
	Description  = "Create a console login profile (password) for an IAM user. Password is sensitive."
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
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name", Placeholder: "jane.doe", Required: true},
	{Name: "password", Type: core.ConnectionTypeSecret, Label: "Password (sensitive)", Required: true},
	{Name: "password_reset_required", Type: core.ConnectionTypeBoolean, Label: "Require password reset on next sign-in (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name"},
	{Name: "create_date", Type: core.ConnectionTypeString, Label: "Create Date"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	userName := strings.TrimSpace(awscommon.InputString("user_name", inputs))
	if userName == "" {
		return nil, fmt.Errorf("user name is required")
	}
	password := awscommon.InputString("password", inputs)
	if password == "" {
		return nil, fmt.Errorf("password is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.CreateLoginProfileInput{
		UserName:              aws.String(userName),
		Password:              aws.String(password),
		PasswordResetRequired: awscommon.InputBool("password_reset_required", inputs),
	}

	out, err := client.CreateLoginProfile(ctx, in)
	if err != nil {
		return nil, err
	}

	var createDate string
	if out.LoginProfile != nil && out.LoginProfile.CreateDate != nil {
		createDate = out.LoginProfile.CreateDate.Format(time.RFC3339)
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created login profile for %s", userName),
		"user_name":   userName,
		"create_date": createDate,
	}, nil
}
