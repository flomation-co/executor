// Package aws_iam_get_user retrieves details about an IAM user.
package aws_iam_get_user

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
	Name         = "AWS IAM Get User"
	Description  = "Retrieve details about an IAM user (blank name returns the calling user)."
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
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name (optional)", Placeholder: "Blank = calling user"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "ARN"},
	{Name: "create_date", Type: core.ConnectionTypeString, Label: "Create Date"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.GetUserInput{}
	if u := strings.TrimSpace(awscommon.InputString("user_name", inputs)); u != "" {
		in.UserName = aws.String(u)
	}

	out, err := client.GetUser(ctx, in)
	if err != nil {
		return nil, err
	}

	var userName, userID, arn, createDate string
	if out.User != nil {
		userName = aws.ToString(out.User.UserName)
		userID = aws.ToString(out.User.UserId)
		arn = aws.ToString(out.User.Arn)
		if out.User.CreateDate != nil {
			createDate = out.User.CreateDate.Format(time.RFC3339)
		}
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("IAM user %s (%s)", userName, arn),
		"user_name":   userName,
		"user_id":     userID,
		"arn":         arn,
		"create_date": createDate,
	}, nil
}
