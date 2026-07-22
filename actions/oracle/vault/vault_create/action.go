// Package oracle_vault_create creates an OCI Vault — the container that holds master
// encryption keys and secrets. Provisioning is synchronous-ish; poll Get Vault until it
// is ACTIVE, at which point its management and crypto endpoints are usable.
package oracle_vault_create

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
	Name         = "OCI Vault: Create Vault"
	Description  = "Create an Oracle Cloud Vault — the container for master encryption keys and secrets. A DEFAULT vault is shared, VIRTUAL_PRIVATE is dedicated. Returns the OCID immediately in a CREATING state; poll Get Vault until ACTIVE."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly name for the vault", Required: true},
	{Name: "vault_type", Type: core.ConnectionTypeString, Label: "Vault Type", Placeholder: "DEFAULT (shared) or VIRTUAL_PRIVATE (dedicated)", Options: []core.ConnectionOption{
		{Name: "Default (shared)", Value: "DEFAULT"},
		{Name: "Virtual Private (dedicated)", Value: "VIRTUAL_PRIVATE"},
	}},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
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
	displayName, err := kms.RequiredString("display_name", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	vaultType := keymanagement.CreateVaultDetailsVaultTypeDefault
	switch strings.ToUpper(strings.TrimSpace(kms.OptionalString("vault_type", inputs))) {
	case "VIRTUAL_PRIVATE":
		vaultType = keymanagement.CreateVaultDetailsVaultTypeVirtualPrivate
	case "", "DEFAULT":
		vaultType = keymanagement.CreateVaultDetailsVaultTypeDefault
	default:
		return kms.ErrorResult("vault type must be DEFAULT or VIRTUAL_PRIVATE"), nil
	}
	details := keymanagement.CreateVaultDetails{CompartmentId: &compartment, DisplayName: &displayName, VaultType: vaultType}
	if tags, err := kms.FreeformTags("tags", inputs); err != nil {
		return kms.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateVault(kms.Context(), keymanagement.CreateVaultRequest{CreateVaultDetails: details})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	vault := kms.SummariseVault(&resp.Vault)
	return kms.Result(fmt.Sprintf("Creating vault %q (%s) — poll Get Vault until ACTIVE", displayName, vault["lifecycle_state"]), map[string]interface{}{
		"vault": vault, "id": vault["id"], "lifecycle_state": vault["lifecycle_state"],
	}), nil
}
