// Package oracle_vault_secret_version_get fetches the metadata for one version of a
// secret — its stages, content type and creation time. This is version METADATA, not the
// secret value; the actual content comes from Get Secret Bundle.
package oracle_vault_secret_version_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	ovault "github.com/oracle/oci-go-sdk/v65/vault"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Get Secret Version"
	Description  = "Fetch the metadata for one version of a secret — its stages, content type and creation time. This is version metadata, not the secret value; retrieve the content with Get Secret Bundle."
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
	{Name: "secret_ocid", Type: core.ConnectionTypeString, Label: "Secret OCID", Placeholder: "ocid1.vaultsecret.oc1..aaaa… whose version to fetch", Required: true},
	{Name: "version_number", Type: core.ConnectionTypeString, Label: "Version Number", Placeholder: "The version number, a whole number (e.g. 1)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "secret_version", Type: core.ConnectionTypeObject, Label: "Secret Version"},
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
	resp, err := client.GetSecretVersion(kms.Context(), ovault.GetSecretVersionRequest{SecretId: &sid, SecretVersionNumber: &vnum})
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	version := kms.SummariseSecretVersion(&resp.SecretVersion)
	return kms.Result(fmt.Sprintf("Secret version %d — content type %q (metadata only; use Get Secret Bundle for the value)", vnum, version["content_type"]), map[string]interface{}{
		"secret_version": version,
	}), nil
}
