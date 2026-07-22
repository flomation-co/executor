// Package aws_kms_create_key creates an AWS KMS key.
package aws_kms_create_key

import (
	"context"
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
	Name         = "AWS KMS Create Key"
	Description  = "Create an AWS KMS key for encryption, signing or MAC generation."
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
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
	{Name: "key_usage", Type: core.ConnectionTypeString, Label: "Key Usage", Options: []core.ConnectionOption{
		{Name: "Encrypt & Decrypt", Value: "ENCRYPT_DECRYPT"},
		{Name: "Sign & Verify", Value: "SIGN_VERIFY"},
		{Name: "Generate & Verify MAC", Value: "GENERATE_VERIFY_MAC"},
	}},
	{Name: "key_spec", Type: core.ConnectionTypeString, Label: "Key Spec (optional)", Options: []core.ConnectionOption{
		{Name: "Symmetric Default", Value: "SYMMETRIC_DEFAULT"},
		{Name: "RSA 2048", Value: "RSA_2048"},
		{Name: "RSA 3072", Value: "RSA_3072"},
		{Name: "RSA 4096", Value: "RSA_4096"},
		{Name: "ECC NIST P256", Value: "ECC_NIST_P256"},
		{Name: "ECC NIST P384", Value: "ECC_NIST_P384"},
		{Name: "ECC NIST P521", Value: "ECC_NIST_P521"},
		{Name: "ECC SECG P256K1", Value: "ECC_SECG_P256K1"},
		{Name: "HMAC 256", Value: "HMAC_256"},
	}},
	{Name: "policy", Type: core.ConnectionTypeString, Label: "Key Policy Document (JSON, optional)", Placeholder: `{"Version":"2012-10-17","Statement":[...]}`},
	{Name: "tags", Type: core.ConnectionTypeKeyValueArray, Label: "Tags", Placeholder: "Add a Key and Value per tag"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID"},
	{Name: "key_arn", Type: core.ConnectionTypeString, Label: "Key ARN"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	in := &kms.CreateKeyInput{}
	if d := strings.TrimSpace(awscommon.InputString("description", inputs)); d != "" {
		in.Description = aws.String(d)
	}
	if u := strings.TrimSpace(awscommon.InputString("key_usage", inputs)); u != "" {
		in.KeyUsage = kmstypes.KeyUsageType(u)
	}
	if s := strings.TrimSpace(awscommon.InputString("key_spec", inputs)); s != "" {
		in.KeySpec = kmstypes.KeySpec(s)
	}
	if p := strings.TrimSpace(awscommon.InputString("policy", inputs)); p != "" {
		in.Policy = aws.String(p)
	}
	if conn := core.FindConnection("tags", inputs); conn != nil {
		for _, kv := range conn.KeyValuePairs() {
			k := strings.TrimSpace(kv.Key)
			if k == "" {
				continue
			}
			in.Tags = append(in.Tags, kmstypes.Tag{TagKey: aws.String(k), TagValue: aws.String(strings.TrimSpace(kv.Value))})
		}
	}

	out, err := client.CreateKey(ctx, in)
	if err != nil {
		return nil, err
	}

	var keyID, keyARN string
	if out.KeyMetadata != nil {
		keyID = aws.ToString(out.KeyMetadata.KeyId)
		keyARN = aws.ToString(out.KeyMetadata.Arn)
	}

	return map[string]interface{}{
		"tool_result": "Created KMS key " + keyID,
		"key_id":      keyID,
		"key_arn":     keyARN,
	}, nil
}
