// Package aws_kms_import_key_material imports wrapped key material into a KMS key.
package aws_kms_import_key_material

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	kmstypes "github.com/aws/aws-sdk-go-v2/service/kms/types"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS KMS Import Key Material"
	Description  = "Import wrapped key material into a KMS key with EXTERNAL origin."
	Website      = "https://www.flomation.co"
	Icon         = "key+arrow-up"
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID / ARN", Placeholder: "The key with EXTERNAL origin", Required: true},
	{Name: "import_token", Type: core.ConnectionTypeString, Label: "Import Token (base64)", Placeholder: "From Get Parameters For Import", Required: true},
	{Name: "encrypted_key_material", Type: core.ConnectionTypeString, Label: "Encrypted Key Material (base64)", Placeholder: "Your key material wrapped with the public key", Required: true},
	{Name: "expiration_model", Type: core.ConnectionTypeString, Label: "Expiration Model (optional)", Options: []core.ConnectionOption{
		{Name: "Key Material Expires", Value: "KEY_MATERIAL_EXPIRES"},
		{Name: "Key Material Does Not Expire", Value: "KEY_MATERIAL_DOES_NOT_EXPIRE"},
	}},
	{Name: "valid_to", Type: core.ConnectionTypeString, Label: "Valid To (RFC3339, optional)", Placeholder: "2026-12-31T23:59:59Z"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}
	importTokenB64 := awscommon.InputString("import_token", inputs)
	if importTokenB64 == "" {
		return nil, fmt.Errorf("import token is required")
	}
	encryptedMaterialB64 := awscommon.InputString("encrypted_key_material", inputs)
	if encryptedMaterialB64 == "" {
		return nil, fmt.Errorf("encrypted key material is required")
	}

	importToken, err := base64.StdEncoding.DecodeString(importTokenB64)
	if err != nil {
		return nil, fmt.Errorf("import token must be valid base64: %w", err)
	}
	encryptedMaterial, err := base64.StdEncoding.DecodeString(encryptedMaterialB64)
	if err != nil {
		return nil, fmt.Errorf("encrypted key material must be valid base64: %w", err)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	in := &kms.ImportKeyMaterialInput{
		KeyId:                aws.String(keyID),
		ImportToken:          importToken,
		EncryptedKeyMaterial: encryptedMaterial,
	}
	if em := strings.TrimSpace(awscommon.InputString("expiration_model", inputs)); em != "" {
		in.ExpirationModel = kmstypes.ExpirationModelType(em)
	}
	if vt := strings.TrimSpace(awscommon.InputString("valid_to", inputs)); vt != "" {
		t, err := time.Parse(time.RFC3339, vt)
		if err != nil {
			return nil, fmt.Errorf("valid_to must be RFC3339: %w", err)
		}
		in.ValidTo = aws.Time(t)
	}

	out, err := client.ImportKeyMaterial(ctx, in)
	if err != nil {
		return nil, err
	}

	outKeyID := aws.ToString(out.KeyId)
	return map[string]interface{}{
		"tool_result": fmt.Sprintf("Imported key material into %s", outKeyID),
		"key_id":      outKeyID,
	}, nil
}
