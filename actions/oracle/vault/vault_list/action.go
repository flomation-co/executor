// Package oracle_vault_list lists the Vaults in a compartment.
package oracle_vault_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	keymanagement "github.com/oracle/oci-go-sdk/v65/keymanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: List Vaults"
	Description  = "List the Oracle Cloud Vaults in a compartment. Walks pagination up to a safe cap."
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vaults", Type: core.ConnectionTypeObject, Label: "Vaults"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
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
	req := keymanagement.ListVaultsRequest{CompartmentId: &compartment}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= kms.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListVaults(kms.Context(), req)
		if err != nil {
			return kms.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, kms.SummariseVaultSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return kms.Result(fmt.Sprintf("Found %d vault(s)", len(out)), map[string]interface{}{
		"vaults": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
