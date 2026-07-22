// Package oracle_vault_key_version_create creates a new key version for a master
// key (rotating it — via the vault's management endpoint).
package oracle_vault_key_version_create

import (
	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Create Key Version"
	Description  = "Create a new key version for a master encryption key in an Oracle Cloud vault (resolved via the vault's management endpoint), rotating the key's active material."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa… holding the key", Required: true},
	{Name: "key_ocid", Type: core.ConnectionTypeString, Label: "Key OCID", Placeholder: "ocid1.key.oc1..aaaa… to create a new version for", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key_version", Type: core.ConnectionTypeObject, Label: "Key version"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Key version OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle state"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, _, errResult := kms.ManagementForVault(inputs, "vault_ocid")
	if errResult != nil {
		return errResult, nil
	}
	kid, err := kms.RequiredString("key_ocid", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	resp, err := client.CreateKeyVersion(kms.Context(), keymanagement.CreateKeyVersionRequest{KeyId: &kid})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	summary := kms.SummariseKeyVersion(&resp.KeyVersion)
	return kms.Result("Created key version", map[string]interface{}{
		"key_version":     summary,
		"id":              summary["id"],
		"lifecycle_state": summary["lifecycle_state"],
	}), nil
}
