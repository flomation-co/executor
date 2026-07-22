// Package aws_kms_get_parameters_for_import retrieves the wrapping key and import token needed to import key material.
package aws_kms_get_parameters_for_import

import (
	"context"
	"encoding/base64"
	"fmt"
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
	Name         = "AWS KMS Get Parameters For Import"
	Description  = "Get the public wrapping key and import token needed to import key material."
	Website      = "https://www.flomation.co"
	Icon         = "key+arrow-down"
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
	{Name: "wrapping_algorithm", Type: core.ConnectionTypeString, Label: "Wrapping Algorithm", Required: true, Options: []core.ConnectionOption{
		{Name: "RSAES OAEP SHA-256", Value: "RSAES_OAEP_SHA_256"},
		{Name: "RSAES OAEP SHA-1", Value: "RSAES_OAEP_SHA_1"},
		{Name: "RSA AES Key Wrap SHA-256", Value: "RSA_AES_KEY_WRAP_SHA_256"},
		{Name: "RSA AES Key Wrap SHA-1", Value: "RSA_AES_KEY_WRAP_SHA_1"},
		{Name: "RSAES PKCS1 v1.5", Value: "RSAES_PKCS1_V1_5"},
	}},
	{Name: "wrapping_key_spec", Type: core.ConnectionTypeString, Label: "Wrapping Key Spec", Required: true, Options: []core.ConnectionOption{
		{Name: "RSA 2048", Value: "RSA_2048"},
		{Name: "RSA 3072", Value: "RSA_3072"},
		{Name: "RSA 4096", Value: "RSA_4096"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "import_token", Type: core.ConnectionTypeString, Label: "Import Token (base64)"},
	{Name: "public_key", Type: core.ConnectionTypeString, Label: "Wrapping Public Key (base64)"},
	{Name: "parameters_valid_to", Type: core.ConnectionTypeString, Label: "Parameters Valid To"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}
	wrappingAlgorithm := awscommon.InputString("wrapping_algorithm", inputs)
	if wrappingAlgorithm == "" {
		return nil, fmt.Errorf("wrapping algorithm is required")
	}
	wrappingKeySpec := awscommon.InputString("wrapping_key_spec", inputs)
	if wrappingKeySpec == "" {
		return nil, fmt.Errorf("wrapping key spec is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	out, err := client.GetParametersForImport(ctx, &kms.GetParametersForImportInput{
		KeyId:             aws.String(keyID),
		WrappingAlgorithm: kmstypes.AlgorithmSpec(wrappingAlgorithm),
		WrappingKeySpec:   kmstypes.WrappingKeySpec(wrappingKeySpec),
	})
	if err != nil {
		return nil, err
	}

	var validTo string
	if out.ParametersValidTo != nil {
		validTo = out.ParametersValidTo.Format(time.RFC3339)
	}
	return map[string]interface{}{
		"tool_result":         fmt.Sprintf("Retrieved import parameters for %s", aws.ToString(out.KeyId)),
		"import_token":        base64.StdEncoding.EncodeToString(out.ImportToken),
		"public_key":          base64.StdEncoding.EncodeToString(out.PublicKey),
		"parameters_valid_to": validTo,
	}, nil
}
