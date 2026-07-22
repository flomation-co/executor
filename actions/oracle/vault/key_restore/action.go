// Package oracle_vault_key_restore restores a master encryption key into a vault from a
// backup held in Object Storage (a bucket or a pre-authenticated URI), via the vault's
// management endpoint. The restore creates a NEW key — there is no key OCID input.
package oracle_vault_key_restore

import (
	"fmt"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Restore Key"
	Description  = "Restore a master encryption key into an Oracle Cloud vault from an Object Storage backup (a bucket or a pre-authenticated URI). Creates a new key."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the vault picker)"},
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa… of the vault to restore the key into", Required: true},
	{Name: "bucket_namespace", Type: core.ConnectionTypeString, Label: "Bucket Namespace", Placeholder: "Object Storage namespace holding the backup"},
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "Bucket holding the key backup"},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Object Name", Placeholder: "Object name of the key backup within the bucket"},
	{Name: "backup_uri", Type: core.ConnectionTypeString, Label: "Backup URI", Placeholder: "Pre-authenticated request URI (alternative to a bucket)"},
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

	var loc keymanagement.BackupLocation
	ns := kms.OptionalString("bucket_namespace", inputs)
	bn := kms.OptionalString("bucket_name", inputs)
	obj := kms.OptionalString("object_name", inputs)
	uri := kms.OptionalString("backup_uri", inputs)
	switch {
	case ns != "" && bn != "" && obj != "":
		loc = &keymanagement.BackupLocationBucket{Namespace: &ns, BucketName: &bn, ObjectName: &obj}
	case uri != "":
		loc = &keymanagement.BackupLocationUri{Uri: &uri}
	default:
		return kms.ErrorResult("provide either bucket_namespace+bucket_name+object_name or a pre-authenticated backup_uri"), nil
	}

	details := keymanagement.RestoreKeyFromObjectStoreDetails{BackupLocation: loc}
	resp, err := client.RestoreKeyFromObjectStore(kms.Context(), keymanagement.RestoreKeyFromObjectStoreRequest{RestoreKeyFromObjectStoreDetails: details})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	key := kms.SummariseKey(&resp.Key)
	return kms.Result(fmt.Sprintf("Restoring key %q (%s) — poll Get Key until ENABLED", key["display_name"], key["lifecycle_state"]), map[string]interface{}{
		"key": key, "id": key["id"], "lifecycle_state": key["lifecycle_state"],
	}), nil
}
