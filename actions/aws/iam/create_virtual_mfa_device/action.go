// Package aws_iam_create_virtual_mfa_device creates a new virtual MFA device.
package aws_iam_create_virtual_mfa_device

import (
	"context"
	"encoding/base64"
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
	Name         = "AWS IAM Create Virtual MFA Device"
	Description  = "Create a virtual MFA device, returning its seed and QR code."
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
	{Name: "virtual_mfa_device_name", Type: core.ConnectionTypeString, Label: "Virtual MFA Device Name", Placeholder: "jane.doe", Required: true},
	{Name: "path", Type: core.ConnectionTypeString, Label: "Path (optional)", Placeholder: "/division_abc/"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "serial_number", Type: core.ConnectionTypeString, Label: "Serial Number"},
	{Name: "base32_string_seed", Type: core.ConnectionTypeString, Label: "Base32 String Seed (sensitive)"},
	{Name: "qr_code_png", Type: core.ConnectionTypeString, Label: "QR Code PNG (base64)"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	deviceName := awscommon.InputString("virtual_mfa_device_name", inputs)
	if deviceName == "" {
		return nil, fmt.Errorf("virtual MFA device name is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.CreateVirtualMFADeviceInput{VirtualMFADeviceName: aws.String(deviceName)}
	if p := strings.TrimSpace(awscommon.InputString("path", inputs)); p != "" {
		in.Path = aws.String(p)
	}

	out, err := client.CreateVirtualMFADevice(ctx, in)
	if err != nil {
		return nil, err
	}

	var serialNumber, seed, qrCode string
	if out.VirtualMFADevice != nil {
		serialNumber = aws.ToString(out.VirtualMFADevice.SerialNumber)
		if len(out.VirtualMFADevice.Base32StringSeed) > 0 {
			seed = string(out.VirtualMFADevice.Base32StringSeed)
		}
		if len(out.VirtualMFADevice.QRCodePNG) > 0 {
			qrCode = base64.StdEncoding.EncodeToString(out.VirtualMFADevice.QRCodePNG)
		}
	}

	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Created virtual MFA device %s (%s)", deviceName, serialNumber),
		"serial_number":      serialNumber,
		"base32_string_seed": seed,
		"qr_code_png":        qrCode,
	}, nil
}
