// Package aws_ec2_get_password_data retrieves the encrypted Windows
// Administrator password for an EC2 instance (ec2:GetPasswordData) and, when the
// key pair's private key is supplied, decrypts it to the plaintext password.
//
// AWS encrypts the password at launch with the PUBLIC half of the key pair the
// instance was launched with (RSA, PKCS#1 v1.5), so decryption needs that key
// pair's PRIVATE key. Decryption is pure Go (crypto/rsa) — no CGO. The password
// data is only present a few minutes after a fresh Windows launch and is empty
// for Linux, for instances whose password has since been changed, or for AMIs
// that disabled password generation — surfaced via the `available` output.
package aws_ec2_get_password_data

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	awscommon "flomation.app/automate/executor/actions/aws"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

const (
	Author       = "Andy Esser"
	Organisation = "Flomation"
	Name         = "AWS EC2 Get Windows Password"
	Description  = "Retrieve and optionally decrypt the Windows Administrator password for an EC2 instance."
	Website      = "https://www.flomation.co"
	Icon         = "key"
	Date         = "24/07/2026"
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
	{Name: "instance_id", Type: core.ConnectionTypeString, Label: "Instance ID", Placeholder: "i-0abc123...", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeString, Label: "Key Pair Private Key (PEM)", Placeholder: "Wire the launch key pair's private key here (or ${secrets.X}) to decrypt; leave blank to return the encrypted data"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "password", Type: core.ConnectionTypeString, Label: "Administrator Password"},
	{Name: "password_data", Type: core.ConnectionTypeString, Label: "Encrypted Password Data"},
	{Name: "available", Type: core.ConnectionTypeBoolean, Label: "Password Available"},
}

func errResult(msg string) (map[string]interface{}, error) {
	return map[string]interface{}{
		"tool_result": msg,
		"password":    "",
		// password_data omitted on error
		"available": false,
		"success":   false,
	}, fmt.Errorf("%s", msg)
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	ctx := context.Background()

	instanceID := strings.TrimSpace(awscommon.InputString("instance_id", inputs))
	if instanceID == "" {
		return errResult("instance_id is required")
	}

	cfg, err := awscommon.ConfigFromInputs(ctx, inputs)
	if err != nil {
		return errResult(fmt.Sprintf("AWS auth failed: %v", err))
	}
	client := ec2.NewFromConfig(cfg)

	out, err := client.GetPasswordData(ctx, &ec2.GetPasswordDataInput{InstanceId: aws.String(instanceID)})
	if err != nil {
		return errResult(fmt.Sprintf("GetPasswordData failed: %v", err))
	}

	encrypted := strings.TrimSpace(aws.ToString(out.PasswordData))
	if encrypted == "" {
		// Not an error: the data simply isn't ready (or the instance isn't a
		// password-generating Windows instance). Let the flow decide (e.g. loop
		// and retry), so return success with available=false.
		return map[string]interface{}{
			"tool_result":   "Password data not yet available (Windows instances publish it a few minutes after launch)",
			"password":      "",
			"password_data": "",
			"available":     false,
			"success":       true,
		}, nil
	}

	result := map[string]interface{}{
		"password_data": encrypted,
		"available":     true,
		"success":       true,
	}

	privateKey := strings.TrimSpace(awscommon.InputString("private_key", inputs))
	if privateKey == "" {
		result["password"] = ""
		result["tool_result"] = "Retrieved encrypted password data (supply the key pair's private key to decrypt)"
		return result, nil
	}

	password, derr := decryptPassword(encrypted, privateKey)
	if derr != nil {
		return errResult(fmt.Sprintf("decrypt failed: %v", derr))
	}
	result["password"] = password
	result["tool_result"] = "Decrypted the Windows Administrator password"
	return result, nil
}

// decryptPassword base64-decodes the GetPasswordData blob and RSA-decrypts it
// (PKCS#1 v1.5) with the launch key pair's private key. Accepts PKCS#1
// ("RSA PRIVATE KEY") and PKCS#8 ("PRIVATE KEY") PEM — the two forms AWS
// CreateKeyPair emits. Pure crypto/rsa; no CGO.
func decryptPassword(encryptedB64, privateKeyPEM string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedB64)
	if err != nil {
		return "", fmt.Errorf("password data is not valid base64: %w", err)
	}

	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return "", fmt.Errorf("private key is not valid PEM")
	}

	// Windows passwords are ONLY encrypted with RSA key pairs — ED25519 key pairs
	// are not supported for Windows instances (Linux SSH only). So a non-RSA key
	// can never decrypt this; say so plainly.
	var priv *rsa.PrivateKey
	if k, e := x509.ParsePKCS1PrivateKey(block.Bytes); e == nil {
		priv = k
	} else if k8, e8 := x509.ParsePKCS8PrivateKey(block.Bytes); e8 == nil {
		rk, ok := k8.(*rsa.PrivateKey)
		if !ok {
			return "", fmt.Errorf("private key is not an RSA key — Windows passwords require an RSA key pair (ED25519 is not supported)")
		}
		priv = rk
	} else {
		return "", fmt.Errorf("unsupported or malformed private key — Windows passwords require an RSA key pair PEM (ED25519 is not supported)")
	}

	plaintext, err := rsa.DecryptPKCS1v15(rand.Reader, priv, ciphertext)
	if err != nil {
		return "", fmt.Errorf("RSA decrypt failed — is this the private key for the instance's launch key pair? (%w)", err)
	}
	return string(plaintext), nil
}
