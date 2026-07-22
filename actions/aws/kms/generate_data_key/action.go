// Package aws_kms_generate_data_key generates a data key protected by a KMS key.
package aws_kms_generate_data_key

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS KMS Generate Data Key"
	Description  = "Generate a symmetric data key returning both plaintext and encrypted forms."
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
	{Name: "encryption_context", Type: core.ConnectionTypeKeyValueArray, Label: "Encryption Context (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "plaintext", Type: core.ConnectionTypeString, Label: "Data Key Plaintext (sensitive)"},
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

	in := &kms.GenerateDataKeyInput{KeyId: aws.String(keyID)}
	if spec := awscommon.InputString("key_spec", inputs); spec != "" {
		in.KeySpec = kmstypes.DataKeySpec(spec)
	} else {
		in.KeySpec = kmstypes.DataKeySpecAes256
	}
	if ec := encryptionContext(inputs); len(ec) > 0 {
		in.EncryptionContext = ec
	}

	out, err := client.GenerateDataKey(ctx, in)
	if err != nil {
		return nil, err
	}

	plaintext := base64.StdEncoding.EncodeToString(out.Plaintext)
	blob := base64.StdEncoding.EncodeToString(out.CiphertextBlob)
	outKeyID := aws.ToString(out.KeyId)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Generated data key under %s", outKeyID),
		"plaintext":       plaintext,
		"ciphertext_blob": blob,
		"key_id":          outKeyID,
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
