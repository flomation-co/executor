// Package oracle_vault_secret_bundle_get retrieves the contents of a secret (a "secret
// bundle") from the secrets retrieval endpoint. Returns the base64-encoded value plus its
// version metadata. By default the current version is returned; pass a version number or
// stage to fetch a specific one.
package oracle_vault_secret_bundle_get

import (
	"strings"

	core "flomation.app/automate/executor"
	kms "flomation.app/automate/executor/actions/oracle/vault"

	secrets "github.com/oracle/oci-go-sdk/v65/secrets"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Vault: Get Secret Bundle"
	Description  = "Retrieve the contents of a secret (its base64-encoded value plus version metadata) from an Oracle Cloud vault. Defaults to the current version; pass a version number or stage (CURRENT/PENDING/LATEST/PREVIOUS/DEPRECATED) to fetch a specific one."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+key"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the secret picker)"},
	{Name: "vault_ocid", Type: core.ConnectionTypeString, Label: "Vault OCID", Placeholder: "ocid1.vault.oc1..aaaa… (scopes the secret picker)"},
	{Name: "secret_ocid", Type: core.ConnectionTypeString, Label: "Secret OCID", Placeholder: "ocid1.vaultsecret.oc1..aaaa… to retrieve", Required: true},
	{Name: "version_number", Type: core.ConnectionTypeString, Label: "Version Number", Placeholder: "A specific version number (optional; defaults to current)"},
	{Name: "stage", Type: core.ConnectionTypeString, Label: "Stage", Placeholder: "CURRENT (default), PENDING, LATEST, PREVIOUS or DEPRECATED", Options: []core.ConnectionOption{
		{Name: "Current", Value: "CURRENT"},
		{Name: "Pending", Value: "PENDING"},
		{Name: "Latest", Value: "LATEST"},
		{Name: "Previous", Value: "PREVIOUS"},
		{Name: "Deprecated", Value: "DEPRECATED"},
	}},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "content", Type: core.ConnectionTypeString, Label: "Secret Content (base64)"},
	{Name: "version_number", Type: core.ConnectionTypeString, Label: "Version Number"},
	{Name: "version_name", Type: core.ConnectionTypeString, Label: "Version Name"},
	{Name: "secret_id", Type: core.ConnectionTypeString, Label: "Secret OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := kms.SecretRetrievalClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	secretID, err := kms.RequiredString("secret_ocid", inputs)
	if err != nil {
		return kms.ErrorResult(err.Error()), nil
	}
	req := secrets.GetSecretBundleRequest{SecretId: &secretID}
	if v, ok, err := kms.OptionalInt("version_number", inputs); err != nil {
		return kms.ErrorResult(err.Error()), nil
	} else if ok {
		n := int64(v)
		req.VersionNumber = &n
	}
	if stage := strings.ToUpper(strings.TrimSpace(kms.OptionalString("stage", inputs))); stage != "" {
		req.Stage = secrets.GetSecretBundleStageEnum(stage)
	}
	resp, err := client.GetSecretBundle(kms.Context(), req)
	if err != nil {
		return kms.ErrorResult(auth.OCIError(err)), nil
	}
	content := ""
	if b, ok := resp.SecretBundleContent.(secrets.Base64SecretBundleContentDetails); ok {
		content = kms.Str(b.Content)
	}
	return kms.Result("Retrieved secret bundle — capture the content", map[string]interface{}{
		"content":        content,
		"version_number": kms.Int64OrNil(resp.VersionNumber),
		"version_name":   kms.Str(resp.VersionName),
		"secret_id":      secretID,
	}), nil
}
