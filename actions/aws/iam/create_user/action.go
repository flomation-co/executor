// Package aws_iam_create_user creates an IAM user.
package aws_iam_create_user

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
	Name         = "AWS IAM Create User"
	Description  = "Create a new IAM user with optional path, permissions boundary and tags."
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
	{Name: "path", Type: core.ConnectionTypeString, Label: "Path (optional)", Placeholder: "/division_abc/"},
	{Name: "permissions_boundary", Type: core.ConnectionTypeString, Label: "Permissions Boundary ARN (optional)", Placeholder: "arn:aws:iam::<account>:policy/Boundary"},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name"},
	{Name: "user_id", Type: core.ConnectionTypeString, Label: "User ID"},
	{Name: "arn", Type: core.ConnectionTypeString, Label: "ARN"},
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

	in := &iam.CreateUserInput{UserName: aws.String(userName)}
	if p := strings.TrimSpace(awscommon.InputString("path", inputs)); p != "" {
		in.Path = aws.String(p)
	}
	if b := strings.TrimSpace(awscommon.InputString("permissions_boundary", inputs)); b != "" {
		in.PermissionsBoundary = aws.String(b)
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

	out, err := client.CreateUser(ctx, in)
	if err != nil {
		return nil, err
	}

	var userID, arn string
	if out.User != nil {
		userID = aws.ToString(out.User.UserId)
		arn = aws.ToString(out.User.Arn)
	}
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Created IAM user %s (%s)", userName, arn),
		"user_name":   userName,
		"user_id":     userID,
		"arn":         arn,
	}, nil
}
