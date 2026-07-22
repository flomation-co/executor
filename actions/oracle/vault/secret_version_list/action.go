// Package oracle_vault_secret_version_list lists the versions of a single secret,
// newest first. Walks pagination up to a safe cap.
package oracle_vault_secret_version_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	ovault "github.com/oracle/oci-go-sdk/v65/vault"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: List Secret Versions"
	Description  = "List the versions of a single Oracle Cloud vault secret, newest first, showing each version's number, rotation stages and creation time. Walks pagination up to a safe cap."
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
	{Name: "secret_ocid", Type: core.ConnectionTypeString, Label: "Secret OCID", Placeholder: "ocid1.vaultsecret.oc1..aaaa… to list versions of", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "versions", Type: core.ConnectionTypeObject, Label: "Secret Versions"},
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
	sid, err := kms.RequiredString("secret_ocid", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	req := ovault.ListSecretVersionsRequest{SecretId: &sid}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= kms.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListSecretVersions(kms.Context(), req)
		if err != nil {
			return kms.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, kms.SummariseSecretVersionSummary(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return kms.Result(fmt.Sprintf("Found %d secret version(s)", len(out)), map[string]interface{}{
		"versions": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
