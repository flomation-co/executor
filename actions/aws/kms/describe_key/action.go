// Package aws_kms_describe_key describes an AWS KMS key.
package aws_kms_describe_key

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
	Name         = "AWS KMS Describe Key"
	Description  = "Describe an AWS KMS key by ID, ARN or alias."
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID, ARN or Alias", Placeholder: "1234abcd-... / arn:aws:kms:... / alias/my-key", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID"},
	{Name: "key_arn", Type: core.ConnectionTypeString, Label: "Key ARN"},
	{Name: "key_state", Type: core.ConnectionTypeString, Label: "Key State"},
	{Name: "key_usage", Type: core.ConnectionTypeString, Label: "Key Usage"},
	{Name: "key_spec", Type: core.ConnectionTypeString, Label: "Key Spec"},
	{Name: "enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	out, err := client.DescribeKey(ctx, &kms.DescribeKeyInput{KeyId: aws.String(keyID)})
	if err != nil {
		return nil, err
	}

	result := map[string]interface{}{}
	if md := out.KeyMetadata; md != nil {
		result["key_id"] = aws.ToString(md.KeyId)
		result["key_arn"] = aws.ToString(md.Arn)
		result["key_state"] = string(md.KeyState)
		result["key_usage"] = string(md.KeyUsage)
		result["key_spec"] = string(md.KeySpec)
		result["enabled"] = md.Enabled
		result["tool_result"] = fmt.Sprintf("Key %s is %s", aws.ToString(md.KeyId), string(md.KeyState))
	} else {
		result["tool_result"] = "Described key " + keyID
	}

	return result, nil
}
