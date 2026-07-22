// Package aws_kms_generate_data_key_pair_without_plaintext generates an asymmetric data key pair without the plaintext private key.
package aws_kms_generate_data_key_pair_without_plaintext

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
	Name         = "AWS KMS Generate Data Key Pair Without Plaintext"
	Description  = "Generate an asymmetric data key pair returning only the encrypted private key."
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
	{Name: "key_pair_spec", Type: core.ConnectionTypeString, Label: "Key Pair Spec", Required: true, Options: []core.ConnectionOption{
		{Name: "RSA 2048", Value: "RSA_2048"},
		{Name: "RSA 3072", Value: "RSA_3072"},
		{Name: "RSA 4096", Value: "RSA_4096"},
		{Name: "ECC NIST P256", Value: "ECC_NIST_P256"},
		{Name: "ECC NIST P384", Value: "ECC_NIST_P384"},
		{Name: "ECC NIST P521", Value: "ECC_NIST_P521"},
		{Name: "SM2 (China Regions)", Value: "SM2"},
	}},
	{Name: "encryption_context", Type: core.ConnectionTypeKeyValueArray, Label: "Encryption Context (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "private_key_ciphertext_blob", Type: core.ConnectionTypeString, Label: "Private Key Ciphertext Blob (base64)"},
	{Name: "public_key", Type: core.ConnectionTypeString, Label: "Public Key (base64)"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}
	keyPairSpec := awscommon.InputString("key_pair_spec", inputs)
	if keyPairSpec == "" {
		return nil, fmt.Errorf("key pair spec is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	in := &kms.GenerateDataKeyPairWithoutPlaintextInput{
		KeyId:       aws.String(keyID),
		KeyPairSpec: kmstypes.DataKeyPairSpec(keyPairSpec),
	}
	if ec := encryptionContext(inputs); len(ec) > 0 {
		in.EncryptionContext = ec
	}

	out, err := client.GenerateDataKeyPairWithoutPlaintext(ctx, in)
	if err != nil {
		return nil, err
	}

	outKeyID := aws.ToString(out.KeyId)
	return map[string]interface{}{
		"tool_result":                 fmt.Sprintf("Generated %s data key pair (no plaintext) under %s", keyPairSpec, outKeyID),
		"private_key_ciphertext_blob": base64.StdEncoding.EncodeToString(out.PrivateKeyCiphertextBlob),
		"public_key":                  base64.StdEncoding.EncodeToString(out.PublicKey),
		"key_id":                      outKeyID,
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
