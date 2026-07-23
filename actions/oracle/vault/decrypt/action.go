// Package oracle_vault_decrypt decrypts ciphertext with the master key that produced it,
// via the vault's CRYPTO endpoint. The ciphertext must be the base64 value returned by
// Encrypt; the recovered plaintext comes back base64-encoded.
package oracle_vault_decrypt

import (
	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Decrypt"
	Description  = "Decrypt ciphertext with the master key that produced it, via the vault's crypto endpoint. Pass the base64 ciphertext returned by Encrypt; the recovered plaintext comes back base64-encoded."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+shield-halved"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the vault/key pickers)"},
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa… (its crypto endpoint is used)", Required: true},
	{Name: "key_ocid", Type: core.ConnectionTypeString, Label: "Key OCID", Placeholder: "ocid1.key.oc1..aaaa… that encrypted the data", Required: true},
	{Name: "ciphertext", Type: core.ConnectionTypeText, Label: "Ciphertext (base64)", Placeholder: "The base64 ciphertext returned by Encrypt", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "plaintext", Type: core.ConnectionTypeString, Label: "Plaintext (base64)"},
	{Name: "plaintext_checksum", Type: core.ConnectionTypeString, Label: "Plaintext checksum"},
	{Name: "key_id", Type: core.ConnectionTypeString, Label: "Key OCID"},
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
	ciphertext, err := kms.RequiredString("ciphertext", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	resp, err := client.Decrypt(kms.Context(), keymanagement.DecryptRequest{
		DecryptDataDetails: keymanagement.DecryptDataDetails{KeyId: &keyID, Ciphertext: &ciphertext},
	})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	return kms.Result("Decrypted — capture the plaintext (base64)", map[string]interface{}{
		"plaintext":          kms.Str(resp.DecryptedData.Plaintext),
		"plaintext_checksum": kms.Str(resp.DecryptedData.PlaintextChecksum),
		"key_id":             keyID,
	}), nil
}
