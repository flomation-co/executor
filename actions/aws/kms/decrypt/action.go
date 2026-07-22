// Package aws_kms_decrypt decrypts a KMS ciphertext blob.
package aws_kms_decrypt

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS KMS Decrypt"
	Description  = "Decrypt a base64 KMS ciphertext blob back to plaintext."
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
	{Name: "ciphertext_blob", Type: core.ConnectionTypeString, Label: "Ciphertext Blob (base64)", Required: true},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID / ARN / Alias (optional)", Placeholder: "alias/my-key"},
	{Name: "encryption_context", Type: core.ConnectionTypeKeyValueArray, Label: "Encryption Context (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "plaintext", Type: core.ConnectionTypeString, Label: "Plaintext (sensitive)"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	blobB64 := awscommon.InputString("ciphertext_blob", inputs)
	if blobB64 == "" {
		return nil, fmt.Errorf("ciphertext blob is required")
	}
	blob, err := base64.StdEncoding.DecodeString(strings.TrimSpace(blobB64))
	if err != nil {
		return nil, fmt.Errorf("ciphertext blob is not valid base64: %w", err)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	in := &kms.DecryptInput{CiphertextBlob: blob}
	if keyID := awscommon.InputString("key_id", inputs); keyID != "" {
		in.KeyId = aws.String(keyID)
	}
	if ec := encryptionContext(inputs); len(ec) > 0 {
		in.EncryptionContext = ec
	}

	out, err := client.Decrypt(ctx, in)
	if err != nil {
		return nil, err
	}

	plaintext := string(out.Plaintext)
	outKeyID := aws.ToString(out.KeyId)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Decrypted %d bytes with %s", len(plaintext), outKeyID),
		"plaintext":   plaintext,
		"key_id":      outKeyID,
	}, nil
}

func encryptionContext(inputs []*core.Connection) map[string]string {
	conn := core.FindConnection("encryption_context", inputs)
	if conn == nil {
		return nil
	}
	out := map[string]string{}
	for _, kv := range conn.KeyValuePairs() {
		k := strings.TrimSpace(kv.Key)
		if k == "" {
			continue
		}
		out[k] = kv.Value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}
