// Package oracle_vault_sign signs a message (or its digest) with a master key, via the
// vault's CRYPTO endpoint. The message must be base64-encoded (≤ 4KB); the signature comes
// back base64-encoded — verify it with Verify. For large data, hash it first and sign the
// DIGEST (set Message Type to DIGEST, matching the hash to the signing algorithm).
package oracle_vault_sign

import (
	"strings"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Sign"
	Description  = "Sign a message (or its digest) with a master key via the vault's crypto endpoint. The message must be base64-encoded (≤ 4KB); the signature returns base64-encoded — verify it with Verify. For large data, hash it and sign the DIGEST."
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
	{Name: "key_ocid", Type: core.ConnectionTypeString, Label: "Key OCID", Placeholder: "ocid1.key.oc1..aaaa… to sign with (RSA or ECDSA)", Required: true},
	{Name: "message", Type: core.ConnectionTypeText, Label: "Message (base64)", Placeholder: "The message or digest to sign, base64-encoded (≤ 4KB)", Required: true},
	{Name: "signing_algorithm", Type: core.ConnectionTypeString, Label: "Signing Algorithm", Placeholder: "Match the key type (RSA or ECDSA) and the digest's hash", Required: true, Options: []core.ConnectionOption{
		{Name: "SHA-224 RSA PKCS PSS", Value: "SHA_224_RSA_PKCS_PSS"},
		{Name: "SHA-256 RSA PKCS PSS", Value: "SHA_256_RSA_PKCS_PSS"},
		{Name: "SHA-384 RSA PKCS PSS", Value: "SHA_384_RSA_PKCS_PSS"},
		{Name: "SHA-512 RSA PKCS PSS", Value: "SHA_512_RSA_PKCS_PSS"},
		{Name: "SHA-224 RSA PKCS1 v1.5", Value: "SHA_224_RSA_PKCS1_V1_5"},
		{Name: "SHA-256 RSA PKCS1 v1.5", Value: "SHA_256_RSA_PKCS1_V1_5"},
		{Name: "SHA-384 RSA PKCS1 v1.5", Value: "SHA_384_RSA_PKCS1_V1_5"},
		{Name: "SHA-512 RSA PKCS1 v1.5", Value: "SHA_512_RSA_PKCS1_V1_5"},
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
	{Name: "signature", Type: core.ConnectionTypeString, Label: "Signature (base64)"},
	{Name: "key_version_id", Type: core.ConnectionTypeString, Label: "Key Version OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, _, errResult := kms.CryptoForVault(inputs, "vault_ocid")
	if errResult != nil {
		return errResult, nil
	}
	keyID, err := kms.RequiredString("key_ocid", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	message, err := kms.RequiredString("message", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	algo, err := kms.RequiredString("signing_algorithm", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	algoEnum := keymanagement.SignDataDetailsSigningAlgorithmEnum(strings.TrimSpace(algo))
	mtEnum := keymanagement.SignDataDetailsMessageTypeEnum(strings.TrimSpace(kms.OptionalString("message_type", inputs)))
	if mtEnum == "" {
		mtEnum = keymanagement.SignDataDetailsMessageTypeRaw
	}
	details := keymanagement.SignDataDetails{
		KeyId:            &keyID,
		Message:          &message,
		SigningAlgorithm: algoEnum,
		MessageType:      mtEnum,
	}
	resp, err := client.Sign(kms.Context(), keymanagement.SignRequest{SignDataDetails: details})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	return kms.Result("Signed — capture the signature (base64)", map[string]interface{}{
		"signature":      kms.Str(resp.SignedData.Signature),
		"key_version_id": kms.Str(resp.SignedData.KeyVersionId),
	}), nil
}
