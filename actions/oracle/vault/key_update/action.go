// Package oracle_vault_key_update renames a master encryption key and/or replaces its
// freeform tags (via the vault's management endpoint). Only the display name and tags are
// mutable through this action.
package oracle_vault_key_update

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
	Name         = "OCI Vault: Update Key"
	Description  = "Update a master encryption key in an Oracle Cloud vault — change its display name and/or replace its freeform tags (resolved via the vault's management endpoint)."
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
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa… the key lives in", Required: true},
	{Name: "key_ocid", Type: core.ConnectionTypeString, Label: "Key OCID", Placeholder: "ocid1.key.oc1..aaaa… to update", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A new friendly name for the key (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} — replaces existing tags (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "key", Type: core.ConnectionTypeObject, Label: "Key"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Key OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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
	details := keymanagement.UpdateKeyDetails{}
	if displayName := strings.TrimSpace(kms.OptionalString("display_name", inputs)); displayName != "" {
		details.DisplayName = &displayName
	}
	if tags, err := kms.FreeformTags("tags", inputs); err != nil {
		return kms.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.UpdateKey(kms.Context(), keymanagement.UpdateKeyRequest{KeyId: &kid, UpdateKeyDetails: details})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	key := kms.SummariseKey(&resp.Key)
	return kms.Result(fmt.Sprintf("Updated key %q (%s)", key["display_name"], key["lifecycle_state"]), map[string]interface{}{
		"key": key, "id": key["id"], "lifecycle_state": key["lifecycle_state"],
	}), nil
}
