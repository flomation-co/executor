// Package oracle_vault_backup backs up a Vault (optionally including its keys) to an
// Object Storage bucket or a pre-authenticated URI.
package oracle_vault_backup

import (
	"fmt"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Back Up Vault"
	Description  = "Back up an Oracle Cloud Vault (optionally including its keys) to an Object Storage bucket or a pre-authenticated URI."
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
	{Name: "bucket_namespace", Type: core.ConnectionTypeString, Label: "Bucket Namespace", Placeholder: "Object Storage namespace holding the destination bucket"},
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "Destination bucket for the backup"},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Object Name", Placeholder: "Optional object name for the backup (OCI names it if blank)"},
	{Name: "backup_uri", Type: core.ConnectionTypeString, Label: "Backup URI", Placeholder: "Pre-authenticated request URI (alternative to a bucket)"},
	{Name: "include_keys", Type: core.ConnectionTypeBoolean, Label: "Include Keys", Placeholder: "Include the vault's keys in the backup (default true)"},
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

	var loc keymanagement.BackupLocation
	ns := kms.OptionalString("bucket_namespace", inputs)
	bn := kms.OptionalString("bucket_name", inputs)
	uri := kms.OptionalString("backup_uri", inputs)
	switch {
	case ns != "" && bn != "":
		bucket := keymanagement.BackupLocationBucket{Namespace: &ns, BucketName: &bn}
		if obj := kms.OptionalString("object_name", inputs); obj != "" {
			bucket.ObjectName = &obj
		}
		loc = &bucket
	case uri != "":
		loc = &keymanagement.BackupLocationUri{Uri: &uri}
	default:
		return kms.ErrorResult("provide either bucket_namespace+bucket_name or a pre-authenticated backup_uri"), nil
	}

	include := kms.OptionalBool("include_keys", inputs, true)
	details := keymanagement.BackupVaultDetails{BackupLocation: loc, IsIncludeKeys: &include}
	resp, err := client.BackupVault(kms.Context(), keymanagement.BackupVaultRequest{VaultId: &id, BackupVaultDetails: details})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	vault := kms.SummariseVault(&resp.Vault)
	return kms.Result(fmt.Sprintf("Vault %q backup started (%s)", vault["display_name"], vault["lifecycle_state"]), map[string]interface{}{
		"vault": vault, "id": vault["id"], "lifecycle_state": vault["lifecycle_state"],
	}), nil
}
