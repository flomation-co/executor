// Package oracle_vault_verify verifies a cryptographic signature against a message, via
// the vault's CRYPTO endpoint. Supply the same key, signing algorithm and message type
// that produced the signature; the result is a plain true/false in is_valid.
package oracle_vault_verify

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Verify Signature"
	Description  = "Verify a cryptographic signature against a message via the vault's crypto endpoint. Provide the base64 message (or its digest), the base64 signature, and the same key, signing algorithm and message type that produced it. Returns is_valid true or false."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the vault/key pickers)"},
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa… (its crypto endpoint is used)", Required: true},
	{Name: "key_ocid", Type: core.ConnectionTypeString, Label: "Key OCID", Placeholder: "ocid1.key.oc1..aaaa… that signed the message", Required: true},
	{Name: "message", Type: core.ConnectionTypeText, Label: "Message (base64)", Placeholder: "The base64-encoded message, or its digest (≤ 4KB)", Required: true},
	{Name: "signature", Type: core.ConnectionTypeText, Label: "Signature (base64)", Placeholder: "The base64-encoded signature to verify", Required: true},
	{Name: "signing_algorithm", Type: core.ConnectionTypeString, Label: "Signing Algorithm", Placeholder: "The algorithm that produced the signature", Required: true, Options: []core.ConnectionOption{
		{Name: "SHA-224 RSA PKCS#1 v1.5", Value: "SHA_224_RSA_PKCS1_V1_5"},
		{Name: "SHA-256 RSA PKCS#1 v1.5", Value: "SHA_256_RSA_PKCS1_V1_5"},
		{Name: "SHA-384 RSA PKCS#1 v1.5", Value: "SHA_384_RSA_PKCS1_V1_5"},
		{Name: "SHA-512 RSA PKCS#1 v1.5", Value: "SHA_512_RSA_PKCS1_V1_5"},
		{Name: "SHA-224 RSA PSS", Value: "SHA_224_RSA_PKCS_PSS"},
		{Name: "SHA-256 RSA PSS", Value: "SHA_256_RSA_PKCS_PSS"},
		{Name: "SHA-384 RSA PSS", Value: "SHA_384_RSA_PKCS_PSS"},
		{Name: "SHA-512 RSA PSS", Value: "SHA_512_RSA_PKCS_PSS"},
		{Name: "ECDSA SHA-256", Value: "ECDSA_SHA_256"},
		{Name: "ECDSA SHA-384", Value: "ECDSA_SHA_384"},
		{Name: "ECDSA SHA-512", Value: "ECDSA_SHA_512"},
	}},
	{Name: "message_type", Type: core.ConnectionTypeString, Label: "Message Type", Placeholder: "RAW (default) or DIGEST", Options: []core.ConnectionOption{
		{Name: "Raw message", Value: "RAW"},
		{Name: "Message digest", Value: "DIGEST"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "is_valid", Type: core.ConnectionTypeBoolean, Label: "Signature Valid"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, _, errResult := kms.CryptoForVault(inputs, "vault_ocid")
	if errResult != nil {
		return errResult, nil
	}
	kid, err := kms.RequiredString("key_ocid", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	message, err := kms.RequiredString("message", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	signature, err := kms.RequiredString("signature", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	algo, err := kms.RequiredString("signing_algorithm", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	msgType := strings.TrimSpace(kms.OptionalString("message_type", inputs))
	if msgType == "" {
		msgType = "RAW"
	}
	details := keymanagement.VerifyDataDetails{
		KeyId:            &kid,
		Message:          &message,
		Signature:        &signature,
		SigningAlgorithm: keymanagement.VerifyDataDetailsSigningAlgorithmEnum(algo),
		MessageType:      keymanagement.VerifyDataDetailsMessageTypeEnum(msgType),
	}
	resp, err := client.Verify(kms.Context(), keymanagement.VerifyRequest{VerifyDataDetails: details})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	valid := false
	if resp.VerifiedData.IsSignatureValid != nil {
		valid = *resp.VerifiedData.IsSignatureValid
	}
	return kms.Result(fmt.Sprintf("Signature valid: %v", valid), map[string]interface{}{
		"is_valid": valid,
	}), nil
}
