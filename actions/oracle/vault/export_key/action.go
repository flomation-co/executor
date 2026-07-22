// Package oracle_vault_export_key wraps and exports a master key's material via the
// vault's CRYPTO endpoint. The key must have been created exportable. You supply your own
// RSA wrapping public key (PEM); OCI returns the key material encrypted (wrapped) with it,
// so only the holder of the matching private key can unwrap it.
package oracle_vault_export_key

import (
	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Export Key"
	Description  = "Wrap and export a master key's material via the vault's crypto endpoint. The key must have been created exportable. Supply your own RSA wrapping public key (PEM); the material returns encrypted with it, so only the matching private key can unwrap it. RSA_OAEP_AES_SHA256 uses AES key-wrap; RSA_OAEP_SHA256 wraps the material directly."
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
	{Name: "key_ocid", Type: core.ConnectionTypeString, Label: "Key OCID", Placeholder: "ocid1.key.oc1..aaaa… to export (must be exportable)", Required: true},
	{Name: "public_key", Type: core.ConnectionTypeText, Label: "Wrapping Public Key (PEM)", Placeholder: "Your 2048/3072/4096-bit RSA public key — full PEM, incl. BEGIN/END lines", Required: true},
	{Name: "algorithm", Type: core.ConnectionTypeString, Label: "Wrapping Algorithm", Placeholder: "RSA_OAEP_AES_SHA256 or RSA_OAEP_SHA256", Required: true, Options: []core.ConnectionOption{
		{Name: "RSA-OAEP AES (SHA-256)", Value: "RSA_OAEP_AES_SHA256"},
		{Name: "RSA-OAEP (SHA-256)", Value: "RSA_OAEP_SHA256"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "encrypted_key", Type: core.ConnectionTypeString, Label: "Encrypted Key (base64)"},
	{Name: "key_version_id", Type: core.ConnectionTypeString, Label: "Key Version OCID"},
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
	publicKey, err := kms.RequiredString("public_key", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	algo, err := kms.RequiredString("algorithm", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	details := keymanagement.ExportKeyDetails{
		KeyId:     &kid,
		Algorithm: keymanagement.ExportKeyDetailsAlgorithmEnum(algo),
		PublicKey: &publicKey,
	}
	resp, err := client.ExportKey(kms.Context(), keymanagement.ExportKeyRequest{ExportKeyDetails: details})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	e := resp.ExportedKeyData
	return kms.Result("Exported wrapped key material — capture encrypted_key", map[string]interface{}{
		"encrypted_key":  kms.Str(e.EncryptedKey),
		"key_version_id": kms.Str(e.KeyVersionId),
	}), nil
}
