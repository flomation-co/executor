// Package aws_iam_enable_mfa_device enables (associates) an MFA device for an IAM user.
package aws_iam_enable_mfa_device

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM Enable MFA Device"
	Description  = "Associate and activate an MFA device for an IAM user."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+circle-check"
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
	{Name: "serial_number", Type: core.ConnectionTypeString, Label: "Serial Number", Placeholder: "arn:aws:iam::<account>:mfa/jane.doe", Required: true},
	{Name: "authentication_code_1", Type: core.ConnectionTypeString, Label: "Authentication Code 1", Placeholder: "123456", Required: true},
	{Name: "authentication_code_2", Type: core.ConnectionTypeString, Label: "Authentication Code 2", Placeholder: "654321", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name"},
	{Name: "serial_number", Type: core.ConnectionTypeString, Label: "Serial Number"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	userName := awscommon.InputString("user_name", inputs)
	if userName == "" {
		return nil, fmt.Errorf("user name is required")
	}
	serialNumber := awscommon.InputString("serial_number", inputs)
	if serialNumber == "" {
		return nil, fmt.Errorf("serial number is required")
	}
	code1 := awscommon.InputString("authentication_code_1", inputs)
	if code1 == "" {
		return nil, fmt.Errorf("authentication code 1 is required")
	}
	code2 := awscommon.InputString("authentication_code_2", inputs)
	if code2 == "" {
		return nil, fmt.Errorf("authentication code 2 is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	_, err = client.EnableMFADevice(ctx, &iam.EnableMFADeviceInput{
		UserName:            aws.String(userName),
		SerialNumber:        aws.String(serialNumber),
		AuthenticationCode1: aws.String(code1),
		AuthenticationCode2: aws.String(code2),
	})
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result":   fmt.Sprintf("Enabled MFA device %s for user %s", serialNumber, userName),
		"user_name":     userName,
		"serial_number": serialNumber,
	}, nil
}
