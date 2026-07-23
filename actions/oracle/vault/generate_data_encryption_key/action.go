// Package oracle_vault_generate_data_encryption_key generates a data encryption key (DEK)
// under a master key, via the vault's CRYPTO endpoint. The DEK comes back wrapped
// (encrypted) with the master key as the ciphertext; the plaintext DEK is only returned
// when include_plaintext_key is true. Use the plaintext DEK to encrypt bulk data locally
// (envelope encryption), then store only the wrapped ciphertext.
package oracle_vault_generate_data_encryption_key

import (
	"strings"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Generate Data Encryption Key"
	Description  = "Generate a data encryption key (DEK) under a master key, via the vault's crypto endpoint — for envelope encryption of larger data. The DEK returns wrapped (encrypted) with the master key as the ciphertext; the plaintext DEK is included only when Include Plaintext Key is on."
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
	{Name: "key_ocid", Type: core.ConnectionTypeString, Label: "Key OCID", Placeholder: "ocid1.key.oc1..aaaa… the master key to wrap the DEK with", Required: true},
	{Name: "algorithm", Type: core.ConnectionTypeString, Label: "Algorithm", Placeholder: "AES (default), RSA or ECDSA", Options: []core.ConnectionOption{
		{Name: "AES (symmetric)", Value: "AES"},
		{Name: "RSA", Value: "RSA"},
		{Name: "ECDSA", Value: "ECDSA"},
	}},
	{Name: "length", Type: core.ConnectionTypeString, Label: "Length (bytes)", Placeholder: "Optional — AES 16/24/32 (default 32); RSA 256/384/512 (default 256)"},
	{Name: "include_plaintext_key", Type: core.ConnectionTypeBoolean, Label: "Include Plaintext Key", Placeholder: "Also return the DEK unencrypted (default true)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ciphertext", Type: core.ConnectionTypeString, Label: "Ciphertext (wrapped DEK)"},
	{Name: "plaintext", Type: core.ConnectionTypeString, Label: "Plaintext (DEK, only if Include Plaintext Key)"},
	{Name: "plaintext_checksum", Type: core.ConnectionTypeString, Label: "Plaintext Checksum"},
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
	algorithm := keymanagement.KeyShapeAlgorithmAes
	switch strings.ToUpper(strings.TrimSpace(kms.OptionalString("algorithm", inputs))) {
	case "RSA":
		algorithm = keymanagement.KeyShapeAlgorithmRsa
	case "ECDSA":
		algorithm = keymanagement.KeyShapeAlgorithmEcdsa
	case "", "AES":
		algorithm = keymanagement.KeyShapeAlgorithmAes
	default:
		return kms.ErrorResult("algorithm must be AES, RSA or ECDSA"), nil
	}
	// Default the length by algorithm — a flat 32 is only valid for AES; RSA needs 256/384/512.
	explicitLen, hasLen, err := kms.OptionalInt("length", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	length := 32
	if algorithm == keymanagement.KeyShapeAlgorithmRsa {
		length = 256
	}
	if hasLen {
		length = explicitLen
	}
	shape := keymanagement.KeyShape{Algorithm: algorithm, Length: &length}
	include := kms.OptionalBool("include_plaintext_key", inputs, true)
	details := keymanagement.GenerateKeyDetails{KeyId: &kid, IncludePlaintextKey: &include, KeyShape: &shape}
	resp, err := client.GenerateDataEncryptionKey(kms.Context(), keymanagement.GenerateDataEncryptionKeyRequest{GenerateKeyDetails: details})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	g := resp.GeneratedKey
	return kms.Result("Generated a data encryption key (DEK)", map[string]interface{}{
		"ciphertext":         kms.Str(g.Ciphertext),
		"plaintext":          kms.Str(g.Plaintext),
		"plaintext_checksum": kms.Str(g.PlaintextChecksum),
	}), nil
}
