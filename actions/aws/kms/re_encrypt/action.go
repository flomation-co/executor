// Package aws_kms_re_encrypt re-encrypts a ciphertext blob under a new KMS key.
package aws_kms_re_encrypt

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
	Name         = "AWS KMS Re-Encrypt"
	Description  = "Re-encrypt a ciphertext blob under a different KMS key without exposing plaintext."
	Website      = "https://www.flomation.co"
	Icon         = "key+copy"
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
	{Name: "destination_key_id", Type: core.ConnectionTypeString, Label: "Destination Key ID / ARN / Alias", Placeholder: "alias/new-key", Required: true},
	{Name: "source_key_id", Type: core.ConnectionTypeString, Label: "Source Key ID / ARN / Alias (optional)", Placeholder: "alias/old-key"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ciphertext_blob", Type: core.ConnectionTypeString, Label: "Ciphertext Blob (base64)"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Destination Key ID"},
	{Name: "source_key_id", Type: core.ConnectionTypeString, Label: "Source Key ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	blobB64 := awscommon.InputString("ciphertext_blob", inputs)
	if blobB64 == "" {
		return nil, fmt.Errorf("ciphertext blob is required")
	}
	destKeyID := awscommon.InputString("destination_key_id", inputs)
	if destKeyID == "" {
		return nil, fmt.Errorf("destination key id is required")
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

	in := &kms.ReEncryptInput{
		CiphertextBlob:   blob,
		DestinationKeyId: aws.String(destKeyID),
	}
	if src := awscommon.InputString("source_key_id", inputs); src != "" {
		in.SourceKeyId = aws.String(src)
	}

	out, err := client.ReEncrypt(ctx, in)
	if err != nil {
		return nil, err
	}

	outBlob := base64.StdEncoding.EncodeToString(out.CiphertextBlob)
	outKeyID := aws.ToString(out.KeyId)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Re-encrypted ciphertext under %s", outKeyID),
		"ciphertext_blob": outBlob,
		"key_id":          outKeyID,
		"source_key_id":   aws.ToString(out.SourceKeyId),
	}, nil
}
