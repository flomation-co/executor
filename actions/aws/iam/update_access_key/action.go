// Package aws_iam_update_access_key activates or deactivates an IAM access key.
package aws_iam_update_access_key

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
	iamtypes "github.com/aws/aws-sdk-go-v2/service/iam/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Update Access Key"
	Description  = "Activate or deactivate an IAM user's access key."
	Website      = "https://www.flomation.co"
	Icon         = "key+pen"
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
	{Name: "access_key_id", Type: core.ConnectionTypeString, Label: "Access Key ID", Placeholder: "AKIA...", Required: true},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status", Required: true, Options: []core.ConnectionOption{
		{Name: "Active", Value: "Active"},
		{Name: "Inactive", Value: "Inactive"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "access_key_id", Type: core.ConnectionTypeString, Label: "Access Key ID"},
	{Name: "status", Type: core.ConnectionTypeString, Label: "Status"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	userName := awscommon.InputString("user_name", inputs)
	if userName == "" {
		return nil, fmt.Errorf("user name is required")
	}
	accessKeyID := awscommon.InputString("access_key_id", inputs)
	if accessKeyID == "" {
		return nil, fmt.Errorf("access key id is required")
	}
	status := awscommon.InputString("status", inputs)
	if status == "" {
		return nil, fmt.Errorf("status is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	if _, err := client.UpdateAccessKey(ctx, &iam.UpdateAccessKeyInput{
		UserName:    aws.String(userName),
		AccessKeyId: aws.String(accessKeyID),
		Status:      iamtypes.StatusType(status),
	}); err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Set access key %s to %s for IAM user %s", accessKeyID, status, userName),
		"access_key_id": accessKeyID,
		"status":        status,
	}, nil
}
