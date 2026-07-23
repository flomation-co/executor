// Package oracle_vault_get_usage reports a Vault's key and key-version counts (HSM and
// software, across all compartments), which is how OCI surfaces a vault's capacity usage.
package oracle_vault_get_usage

import (
	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Get Vault Usage"
	Description  = "Report how many keys and key versions a Vault holds — HSM and software counts, across all compartments (excluding deleted)."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+lock"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the vault picker)"},
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key_count", Type: core.ConnectionTypeInteger, Label: "Key Count (HSM)"},
	{Name: "key_version_count", Type: core.ConnectionTypeInteger, Label: "Key Version Count (HSM)"},
	{Name: "software_key_count", Type: core.ConnectionTypeInteger, Label: "Software Key Count"},
	{Name: "software_key_version_count", Type: core.ConnectionTypeInteger, Label: "Software Key Version Count"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := kms.VaultClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := kms.RequiredString("vault_ocid", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetVaultUsage(kms.Context(), keymanagement.GetVaultUsageRequest{VaultId: &id})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	u := resp.VaultUsage
	iv := func(p *int) interface{} {
		if p == nil {
			return nil
		}
		return *p
	}
	return kms.Result("Vault usage", map[string]interface{}{
		"key_count":                  iv(u.KeyCount),
		"key_version_count":          iv(u.KeyVersionCount),
		"software_key_count":         iv(u.SoftwareKeyCount),
		"software_key_version_count": iv(u.SoftwareKeyVersionCount),
	}), nil
}
