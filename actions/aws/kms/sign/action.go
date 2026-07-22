// Package aws_kms_sign signs a message with a KMS asymmetric key.
package aws_kms_sign

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
	Name         = "AWS KMS Sign"
	Description  = "Sign a message with a KMS asymmetric key, returning a base64 signature."
	Website      = "https://www.flomation.co"
	Icon         = "key+pen"
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
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID / ARN / Alias", Placeholder: "alias/my-signing-key", Required: true},
	{Name: "message", Type: core.ConnectionTypeString, Label: "Message", Placeholder: "Message to sign", Required: true},
	{Name: "signing_algorithm", Type: core.ConnectionTypeString, Label: "Signing Algorithm", Required: true, Options: []core.ConnectionOption{
		{Name: "RSASSA PSS SHA-256", Value: "RSASSA_PSS_SHA_256"},
		{Name: "RSASSA PKCS1 v1.5 SHA-256", Value: "RSASSA_PKCS1_V1_5_SHA_256"},
		{Name: "ECDSA SHA-256", Value: "ECDSA_SHA_256"},
	}},
	{Name: "message_type", Type: core.ConnectionTypeString, Label: "Message Type", Options: []core.ConnectionOption{
		{Name: "Raw", Value: "RAW"},
		{Name: "Digest", Value: "DIGEST"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "signature", Type: core.ConnectionTypeString, Label: "Signature (base64)"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key ID"},
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
	signingAlgorithm := strings.TrimSpace(awscommon.InputString("signing_algorithm", inputs))
	if signingAlgorithm == "" {
		return nil, fmt.Errorf("signing algorithm is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return nil, err
	}
	client := kms.NewFromConfig(cfg)

	in := &kms.SignInput{
		KeyId:            aws.String(keyID),
		Message:          []byte(message),
		SigningAlgorithm: kmstypes.SigningAlgorithmSpec(signingAlgorithm),
	}
	messageType := strings.TrimSpace(awscommon.InputString("message_type", inputs))
	if messageType == "" {
		messageType = "RAW"
	}
	in.MessageType = kmstypes.MessageType(messageType)

	out, err := client.Sign(ctx, in)
	if err != nil {
		return nil, err
	}

	outKeyID := aws.ToString(out.KeyId)
	return map[string]interface{}{
		"tool_result": "Signed message with " + outKeyID,
		"signature":   base64.StdEncoding.EncodeToString(out.Signature),
		"key_id":      outKeyID,
	}, nil
}
