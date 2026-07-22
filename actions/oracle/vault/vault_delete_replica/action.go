// Package oracle_vault_delete_replica removes a cross-region replica of a Vault. The
// call is asynchronous — OCI returns a work request and tears the replica down in the
// background.
package oracle_vault_delete_replica

import (
	"fmt"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Delete Vault Replica"
	Description  = "Remove a cross-region replica of an Oracle Cloud Vault — asynchronous; OCI tears the replica down in the background."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the vault picker)"},
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa…", Required: true},
	{Name: "replica_region", Type: core.ConnectionTypeString, Label: "Replica Region", Placeholder: "The region whose replica to remove, e.g. uk-cardiff-1", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Vault OCID"},
	{Name: "replica_region", Type: core.ConnectionTypeString, Label: "Replica Region"},
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
	region, err := kms.RequiredString("replica_region", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	_, err = client.DeleteVaultReplica(kms.Context(), keymanagement.DeleteVaultReplicaRequest{
		VaultId: &id,
		DeleteVaultReplicaDetails: keymanagement.DeleteVaultReplicaDetails{
			ReplicaRegion: &region,
		},
	})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	return kms.Result(fmt.Sprintf("Removing vault replica in %s — asynchronous", region), map[string]interface{}{
		"id": id, "replica_region": region,
	}), nil
}
