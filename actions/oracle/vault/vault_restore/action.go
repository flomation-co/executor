// Package oracle_vault_restore restores an OCI Vault from an Object Storage backup. There
// is no vault OCID — the backup is restored into a BRAND-NEW vault in the chosen
// compartment. The backup source is either an Object Storage bucket location (namespace +
// bucket + object) or a pre-authenticated request (PAR) URI. Provisioning is
// synchronous-ish; poll Get Vault until the restored vault is ACTIVE.
package oracle_vault_restore

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
	Name         = "OCI Vault: Restore Vault"
	Description  = "Restore an Oracle Cloud Vault from an Object Storage backup into a new vault — supply either the bucket location (namespace, bucket, object) or a pre-authenticated request URI. Returns the OCID immediately; poll Get Vault until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+lock"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (the compartment to restore the vault into)", Required: true},
	{Name: "bucket_namespace", Type: core.ConnectionTypeString, Label: "Bucket Namespace", Placeholder: "Object Storage namespace holding the backup (with bucket + object)"},
	{Name: "bucket_name", Type: core.ConnectionTypeString, Label: "Bucket Name", Placeholder: "Bucket that holds the backup object (with namespace + object)"},
	{Name: "object_name", Type: core.ConnectionTypeString, Label: "Object Name", Placeholder: "Name of the backup object in the bucket (with namespace + bucket)"},
	{Name: "backup_uri", Type: core.ConnectionTypeString, Label: "Backup PAR URI", Placeholder: "Pre-authenticated request URI to the backup (alternative to the bucket fields)"},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	namespace := strings.TrimSpace(kms.OptionalString("bucket_namespace", inputs))
	bucketName := strings.TrimSpace(kms.OptionalString("bucket_name", inputs))
	objectName := strings.TrimSpace(kms.OptionalString("object_name", inputs))
	backupURI := strings.TrimSpace(kms.OptionalString("backup_uri", inputs))
	var loc keymanagement.BackupLocation
	switch {
	case backupURI != "":
		loc = keymanagement.BackupLocationUri{Uri: &backupURI}
	case namespace != "" && bucketName != "" && objectName != "":
		loc = keymanagement.BackupLocationBucket{Namespace: &namespace, BucketName: &bucketName, ObjectName: &objectName}
	default:
		return kms.ErrorResult("provide either the Object Storage bucket location (namespace, bucket name, and object name) or a pre-authenticated request (PAR) URI to restore from"), nil
	}
	details := keymanagement.RestoreVaultFromObjectStoreDetails{BackupLocation: loc}
	resp, err := client.RestoreVaultFromObjectStore(kms.Context(), keymanagement.RestoreVaultFromObjectStoreRequest{
		CompartmentId:                      &compartment,
		RestoreVaultFromObjectStoreDetails: details,
	})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	vault := kms.SummariseVault(&resp.Vault)
	return kms.Result(fmt.Sprintf("Restoring vault %q (%s) — poll Get Vault until ACTIVE", vault["display_name"], vault["lifecycle_state"]), map[string]interface{}{
		"vault": vault, "id": vault["id"], "lifecycle_state": vault["lifecycle_state"],
	}), nil
}
