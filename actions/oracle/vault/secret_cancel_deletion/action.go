// Package oracle_vault_secret_cancel_deletion reverses a pending secret deletion,
// returning the secret to ACTIVE before its scheduled deletion time elapses.
package oracle_vault_secret_cancel_deletion

import (
	"fmt"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	ovault "github.com/oracle/oci-go-sdk/v65/vault"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Cancel Secret Deletion"
	Description  = "Cancel a pending secret deletion in an Oracle Cloud vault, returning the secret to ACTIVE before its scheduled deletion time elapses."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "secret_ocid", Type: core.ConnectionTypeString, Label: "Secret OCID", Placeholder: "ocid1.vaultsecret.oc1..aaaa… whose pending deletion to cancel", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "secret", Type: core.ConnectionTypeObject, Label: "Secret"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Secret OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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
	_, err = client.CancelSecretDeletion(kms.Context(), ovault.CancelSecretDeletionRequest{SecretId: &sid})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	return kms.Result(fmt.Sprintf("Cancelled pending deletion of secret %s — it should return to ACTIVE", sid), map[string]interface{}{
		"id": sid,
	}), nil
}
