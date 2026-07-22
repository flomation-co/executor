// Package oracle_vault_secret_list lists the secrets in a compartment, optionally
// filtered to a single vault and/or an exact secret name.
package oracle_vault_secret_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	ovault "github.com/oracle/oci-go-sdk/v65/vault"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: List Secrets"
	Description  = "List the secrets in an Oracle Cloud compartment, optionally filtered to a single vault and/or an exact secret name. Walks pagination up to a safe cap."
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
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa… to filter to (optional)"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Secret Name", Placeholder: "Filter to an exact secret name (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "secrets", Type: core.ConnectionTypeObject, Label: "Secrets"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := kms.SecretsMgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	req := ovault.ListSecretsRequest{CompartmentId: &compartment}
	if v := kms.OptionalString("vault_ocid", inputs); v != "" {
		req.VaultId = &v
	}
	if n := kms.OptionalString("name", inputs); n != "" {
		req.Name = &n
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= kms.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListSecrets(kms.Context(), req)
		if err != nil {
			return kms.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, kms.SummariseSecretSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return kms.Result(fmt.Sprintf("Found %d secret(s)", len(out)), map[string]interface{}{
		"secrets": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
