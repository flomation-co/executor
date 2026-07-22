// Package oracle_vault_wrapping_key_get fetches a vault's RSA wrapping key (via the
// vault's management endpoint). The wrapping key's public half is used to wrap external
// key material before importing it into the vault with Import Key.
package oracle_vault_wrapping_key_get

import (
	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Get Wrapping Key"
	Description  = "Fetch an Oracle Cloud vault's RSA wrapping key (resolved via the vault's management endpoint) — use its public key to wrap external key material before importing it with Import Key."
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
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa… to fetch the wrapping key for", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "public_key", Type: core.ConnectionTypeString, Label: "Public Key (PEM)"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Wrapping Key OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, _, errResult := kms.ManagementForVault(inputs, "vault_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetWrappingKey(kms.Context(), keymanagement.GetWrappingKeyRequest{})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	w := resp.WrappingKey
	return kms.Result("Wrapping key for this vault — use it to wrap external key material for Import Key", map[string]interface{}{
		"public_key":      kms.Str(w.PublicKey),
		"id":              kms.Str(w.Id),
		"lifecycle_state": string(w.LifecycleState),
	}), nil
}
