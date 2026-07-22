// Package aws_kms_generate_data_key_without_plaintext generates an encrypted-only data key.
package aws_kms_generate_data_key_without_plaintext

import (
	"context"
	"encoding/base64"
	"fmt"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS KMS Generate Data Key (Encrypted Only)"
	Description  = "Generate a data key returning only its encrypted form, never the plaintext."
	Website      = "https://www.flomation.co"
	Icon         = "key+plus"
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID / ARN / Alias", Placeholder: "alias/my-key", Required: true},
	{Name: "key_spec", Type: core.ConnectionTypeString, Label: "Key Spec (optional)", Options: []core.ConnectionOption{
		{Name: "AES-256", Value: "AES_256"},
		{Name: "AES-128", Value: "AES_128"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ciphertext_blob", Type: core.ConnectionTypeString, Label: "Ciphertext Blob (base64)"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID"},
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

	in := &kms.GenerateDataKeyWithoutPlaintextInput{KeyId: aws.String(keyID)}
	if spec := awscommon.InputString("key_spec", inputs); spec != "" {
		in.KeySpec = kmstypes.DataKeySpec(spec)
	} else {
		in.KeySpec = kmstypes.DataKeySpecAes256
	}

	out, err := client.GenerateDataKeyWithoutPlaintext(ctx, in)
	if err != nil {
		return nil, err
	}

	blob := base64.StdEncoding.EncodeToString(out.CiphertextBlob)
	outKeyID := aws.ToString(out.KeyId)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Generated encrypted data key under %s", outKeyID),
		"ciphertext_blob": blob,
		"key_id":          outKeyID,
	}, nil
}
