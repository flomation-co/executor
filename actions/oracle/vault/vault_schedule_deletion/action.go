// Package oracle_vault_schedule_deletion schedules an Oracle Cloud Vault for deletion,
// moving it to PENDING_DELETION. The vault is not removed until the deletion time (7–30
// days out; 30 days by default) and can be cancelled before then.
package oracle_vault_schedule_deletion

import (
	"fmt"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Schedule Vault Deletion"
	Description  = "Schedule an Oracle Cloud Vault for deletion — it moves to pending-deletion and is removed at the chosen time (7–30 days out; 30 days by default) unless cancelled first."
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
	{Name: "time_of_deletion", Type: core.ConnectionTypeString, Label: "Time of Deletion (RFC3339)", Placeholder: "e.g. 2026-08-15T00:00:00Z — 7–30 days out; leave blank for 30 days"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vault", Type: core.ConnectionTypeObject, Label: "Vault"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Vault OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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
	details := keymanagement.ScheduleVaultDeletionDetails{}
	t, ok, err := kms.ParseSDKTime("time_of_deletion", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	if ok {
		details.TimeOfDeletion = t
	}
	resp, err := client.ScheduleVaultDeletion(kms.Context(), keymanagement.ScheduleVaultDeletionRequest{VaultId: &id, ScheduleVaultDeletionDetails: details})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	vault := kms.SummariseVault(&resp.Vault)
	return kms.Result(fmt.Sprintf("Vault %q scheduled for deletion — now %s", vault["display_name"], vault["lifecycle_state"]), map[string]interface{}{
		"vault": vault, "id": vault["id"], "lifecycle_state": vault["lifecycle_state"],
	}), nil
}
