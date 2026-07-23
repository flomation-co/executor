// Package oracle_certificates_ca_bundle_content_get reads a CA bundle's contents from the
// certificates data plane — the concatenated root/intermediate certificate PEM — by CA bundle OCID.
package oracle_certificates_ca_bundle_content_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificates"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: Get CA Bundle Contents"
	Description  = "Read a CA bundle's contents (the root and intermediate certificate PEM) from the certificates data plane by its OCID."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+id-badge"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "ca_bundle_ocid", Type: core.ConnectionTypeString, Label: "CA Bundle OCID", Placeholder: "ocid1.cabundle.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "bundle", Type: core.ConnectionTypeObject, Label: "CA Bundle Contents"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "CA Bundle OCID"},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name"},
	{Name: "ca_bundle_pem", Type: core.ConnectionTypeString, Label: "CA Bundle PEM"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := certs.DataClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	bundleID, err := certs.RequiredString("ca_bundle_ocid", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetCaBundle(certs.Context(), certificates.GetCaBundleRequest{CaBundleId: &bundleID})
	if err != nil {
		return certs.ErrorResult(auth.OCIError(err)), nil
	}

	bundle := map[string]interface{}{
		"id":            certs.Str(resp.Id),
		"name":          certs.Str(resp.Name),
		"ca_bundle_pem": certs.Str(resp.CaBundlePem),
	}
	return certs.Result(fmt.Sprintf("Read CA bundle contents for %q", bundle["name"]), map[string]interface{}{
		"bundle":        bundle,
		"id":            bundle["id"],
		"name":          bundle["name"],
		"ca_bundle_pem": bundle["ca_bundle_pem"],
	}), nil
}
