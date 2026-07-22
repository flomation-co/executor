// Package aws_kms_get_key_rotation_status reports the rotation status of a KMS key.
package aws_kms_get_key_rotation_status

import (
	"context"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS KMS Get Key Rotation Status"
	Description  = "Report whether automatic rotation is enabled for a KMS key."
	Website      = "https://www.flomation.co"
	Icon         = "key+magnifying-glass"
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID or ARN", Placeholder: "1234abcd-12ab-34cd-56ef-1234567890ab", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key_rotation_enabled", Type: core.ConnectionTypeBoolean, Label: "Rotation Enabled"},
	{Name: "rotation_period_in_days", Type: core.ConnectionTypeInteger, Label: "Rotation Period (days)"},
	{Name: "next_rotation_date", Type: core.ConnectionTypeString, Label: "Next Rotation Date"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	out, err := client.GetKeyRotationStatus(ctx, &kms.GetKeyRotationStatusInput{KeyId: aws.String(keyID)})
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{
		"key_rotation_enabled": out.KeyRotationEnabled,
	}
	if out.RotationPeriodInDays != nil {
		result["rotation_period_in_days"] = int(aws.ToInt32(out.RotationPeriodInDays))
	}
	if out.NextRotationDate != nil {
		result["next_rotation_date"] = out.NextRotationDate.Format("2006-01-02T15:04:05Z07:00")
	}
	result["tool_result"] = fmt.Sprintf("Key rotation for %s is %v", keyID, map[bool]string{true: "enabled", false: "disabled"}[out.KeyRotationEnabled])

	return result, nil
}
