// Package aws_kms_verify_mac verifies an HMAC using a KMS HMAC key.
package aws_kms_verify_mac

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
	Name         = "AWS KMS Verify MAC"
	Description  = "Verify an HMAC for a message using a KMS HMAC key."
	Website      = "https://www.flomation.co"
	Icon         = "key+circle-check"
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID / ARN / Alias", Placeholder: "alias/my-hmac-key", Required: true},
	{Name: "message", Type: core.ConnectionTypeString, Label: "Message", Placeholder: "The original message that was hashed", Required: true},
	{Name: "mac", Type: core.ConnectionTypeString, Label: "MAC (base64)", Placeholder: "The MAC to verify", Required: true},
	{Name: "mac_algorithm", Type: core.ConnectionTypeString, Label: "MAC Algorithm", Required: true, Options: []core.ConnectionOption{
		{Name: "HMAC SHA-256", Value: "HMAC_SHA_256"},
		{Name: "HMAC SHA-384", Value: "HMAC_SHA_384"},
		{Name: "HMAC SHA-512", Value: "HMAC_SHA_512"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "mac_valid", Type: core.ConnectionTypeBoolean, Label: "MAC Valid"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	keyID := awscommon.InputString("key_id", inputs)
	if keyID == "" {
		return nil, fmt.Errorf("key id is required")
	}
	message := awscommon.InputString("message", inputs)
	if message == "" {
		return nil, fmt.Errorf("message is required")
	}
	macB64 := awscommon.InputString("mac", inputs)
	if macB64 == "" {
		return nil, fmt.Errorf("mac is required")
	}
	macAlgorithm := awscommon.InputString("mac_algorithm", inputs)
	if macAlgorithm == "" {
		return nil, fmt.Errorf("mac algorithm is required")
	}

	mac, err := base64.StdEncoding.DecodeString(macB64)
	if err != nil {
		return nil, fmt.Errorf("mac must be valid base64: %w", err)
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	out, err := client.VerifyMac(ctx, &kms.VerifyMacInput{
		KeyId:        aws.String(keyID),
		Message:      []byte(message),
		Mac:          mac,
		MacAlgorithm: kmstypes.MacAlgorithmSpec(macAlgorithm),
	})
	if err != nil {
		return nil, err
	}

	result := "MAC verification failed"
	if out.MacValid {
		result = "MAC verified successfully"
	}
	return map[string]interface{}{
		"tool_result": result,
		"mac_valid":   out.MacValid,
	}, nil
}
