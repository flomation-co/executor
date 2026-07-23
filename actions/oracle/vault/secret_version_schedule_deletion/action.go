// Package oracle_vault_secret_version_schedule_deletion schedules a specific version of a
// secret for deletion in an Oracle Cloud vault. The version moves to pending-deletion and
// is removed at the chosen time (7–30 days out; 30 days by default) unless cancelled first.
package oracle_vault_secret_version_schedule_deletion

import (
	"fmt"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	ovault "github.com/oracle/oci-go-sdk/v65/vault"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Schedule Secret Version Deletion"
	Description  = "Schedule a specific version of a secret in an Oracle Cloud vault for deletion — it moves to pending-deletion and is removed at the chosen time (7–30 days out; 30 days by default) unless cancelled first."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "secret_ocid", Type: core.ConnectionTypeString, Label: "Secret OCID", Placeholder: "ocid1.vaultsecret.oc1..aaaa… whose version to schedule for deletion", Required: true},
	{Name: "version_number", Type: core.ConnectionTypeString, Label: "Version Number", Placeholder: "The version number, a whole number (e.g. 1)", Required: true},
	{Name: "time_of_deletion", Type: core.ConnectionTypeString, Label: "Time of Deletion (RFC3339)", Placeholder: "e.g. 2026-08-15T00:00:00Z — 7–30 days out; leave blank for 30 days"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "secret_version", Type: core.ConnectionTypeObject, Label: "Secret Version"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Secret OCID"},
	{Name: "version_number", Type: core.ConnectionTypeString, Label: "Version Number"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := kms.SecretsMgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	sid, err := kms.RequiredString("secret_ocid", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	vnum, err := kms.RequiredInt64("version_number", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	details := ovault.ScheduleSecretVersionDeletionDetails{}
	t, ok, err := kms.ParseSDKTime("time_of_deletion", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	if ok {
		details.TimeOfDeletion = t
	}
	_, err = client.ScheduleSecretVersionDeletion(kms.Context(), ovault.ScheduleSecretVersionDeletionRequest{
		SecretId: &sid, SecretVersionNumber: &vnum, ScheduleSecretVersionDeletionDetails: details,
	})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	return kms.Result(fmt.Sprintf("Scheduled version %d of secret %s for deletion — it moves to pending-deletion and is removed at the chosen time (7–30 days out; 30 days by default) unless cancelled first", vnum, sid), map[string]interface{}{
		"id": sid, "version_number": vnum,
	}), nil
}
