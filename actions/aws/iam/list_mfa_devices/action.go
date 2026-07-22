// Package aws_iam_list_mfa_devices lists the MFA devices attached to an IAM user.
package aws_iam_list_mfa_devices

import (
	"context"
	"encoding/json"
	"strconv"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/iam"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS IAM List MFA Devices"
	Description  = "List the MFA devices attached to an IAM user (or the caller)."
	Website      = "https://www.flomation.co"
	Icon         = "shield-halved+list"
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
	{Name: "user_name", Type: core.ConnectionTypeString, Label: "User Name (optional)", Placeholder: "jane.doe"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "mfa_devices", Type: core.ConnectionTypeString, Label: "MFA Devices (JSON)"},
	{Name: "count", Type: core.ConnectionTypeInteger, Label: "Count"},
}

type mfaDeviceOut struct {
	SerialNumber string `json:"serial_number"`
	EnableDate   string `json:"enable_date"`
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := iam.NewFromConfig(cfg)

	in := &iam.ListMFADevicesInput{}
	userName := awscommon.InputString("user_name", inputs)
	if userName != "" {
		in.UserName = aws.String(userName)
	}

	var devices []mfaDeviceOut
	for {
		out, err := client.ListMFADevices(ctx, in)
		if err != nil {
			return nil, err
		}
		for _, d := range out.MFADevices {
			var enable string
			if d.EnableDate != nil {
				enable = d.EnableDate.UTC().Format("2006-01-02T15:04:05Z")
			}
			devices = append(devices, mfaDeviceOut{
				SerialNumber: aws.ToString(d.SerialNumber),
				EnableDate:   enable,
			})
		}
		if !out.IsTruncated || out.Marker == nil {
			break
		}
		in.Marker = out.Marker
	}

	encoded, err := json.Marshal(devices)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"tool_result": "Found " + strconv.Itoa(len(devices)) + " MFA device(s)",
		"mfa_devices": string(encoded),
		"count":       len(devices),
	}, nil
}
