// Package oracle_certificates_ca_bundle_update applies a partial update to an OCI Certificates CA
// bundle: only the description and PEM you supply are changed; blank fields are left unchanged.
package oracle_certificates_ca_bundle_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: Update CA Bundle"
	Description  = "Partially update a CA bundle — change only the description or certificates (PEM) you supply; blank fields are left unchanged."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+id-badge"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa…", Required: true},
	{Name: "ca_bundle_ocid", Type: core.ConnectionTypeString, Label: "CA Bundle OCID", Placeholder: "ocid1.certificateauthoritybundle.oc1..aaaa… — the CA bundle to update", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep unchanged)"},
	{Name: "ca_bundle_pem", Type: core.ConnectionTypeText, Label: "CA Bundle (PEM)", Placeholder: "Certificates in PEM format to include in the bundle (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ca_bundle", Type: core.ConnectionTypeObject, Label: "CA Bundle"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "CA Bundle OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := certs.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	caBundleID, err := certs.RequiredString("ca_bundle_ocid", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the fields the operator actually supplied. Blank fields stay nil so
	// the CA bundle keeps its existing values.
	details := certificatesmanagement.UpdateCaBundleDetails{}
	if v := certs.OptionalString("description", inputs); v != "" {
		details.Description = &v
	}
	if v := certs.OptionalString("ca_bundle_pem", inputs); v != "" {
		details.CaBundlePem = &v
	}

	resp, err := client.UpdateCaBundle(certs.Context(), certificatesmanagement.UpdateCaBundleRequest{
		CaBundleId:            &caBundleID,
		UpdateCaBundleDetails: details,
	})
	if err != nil {
		return certs.ErrorResult(auth.OCIError(err)), nil
	}

	bundle := certs.SummariseCaBundle(&resp.CaBundle)
	return certs.Result(fmt.Sprintf("Updated CA bundle %q (%s)", bundle["name"], bundle["lifecycle_state"]), map[string]interface{}{
		"ca_bundle": bundle,
		"id":        bundle["id"],
	}), nil
}
