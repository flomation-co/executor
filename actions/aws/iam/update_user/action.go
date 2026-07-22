// Package aws_iam_update_user renames or repaths an IAM user.
package aws_iam_update_user

import (
	"context"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Update User"
	Description  = "Rename an IAM user or change its path."
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
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name", Placeholder: "jane.doe", Required: true},
	{Name: "new_user_name", Type: core.ConnectionTypeString, Label: "New User Name (optional)", Placeholder: "jane.smith"},
	{Name: "new_path", Type: core.ConnectionTypeString, Label: "New Path (optional)", Placeholder: "/division_abc/"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	userName := awscommon.InputString("user_name", inputs)
	if userName == "" {
		return nil, fmt.Errorf("user name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.UpdateUserInput{UserName: aws.String(userName)}
	resultName := userName
	if n := strings.TrimSpace(awscommon.InputString("new_user_name", inputs)); n != "" {
		in.NewUserName = aws.String(n)
		resultName = n
	}
	if p := strings.TrimSpace(awscommon.InputString("new_path", inputs)); p != "" {
		in.NewPath = aws.String(p)
	}

	if _, err := client.UpdateUser(ctx, in); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Updated IAM user %s", resultName),
		"user_name":   resultName,
	}, nil
}
