// Package oracle_certificates_ca_bundle_create creates a CA bundle: a named, PEM-encoded set of
// certificate-authority certificates (a trust store) that other OCI services can reference to
// verify peer certificates.
package oracle_certificates_ca_bundle_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: Create CA Bundle"
	Description  = "Create a CA bundle — a named, PEM-encoded set of certificate-authority certificates (a trust store) in a compartment."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Name", Placeholder: "A unique name for the CA bundle within the compartment", Required: true},
	{Name: "ca_bundle_pem", Type: core.ConnectionTypeText, Label: "CA Bundle (PEM)", Placeholder: "One or more certificates in PEM format, incl. BEGIN/END CERTIFICATE lines", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "ca_bundle", Type: core.ConnectionTypeObject, Label: "CA Bundle"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "CA Bundle OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := certs.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}
	name, err := certs.RequiredString("name", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}
	pem, err := certs.RequiredString("ca_bundle_pem", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}

	details := certificatesmanagement.CreateCaBundleDetails{
		Name:          &name,
		CompartmentId: &compartment,
		CaBundlePem:   &pem,
	}
	if d := certs.OptionalString("description", inputs); d != "" {
		details.Description = &d
	}

	resp, err := client.CreateCaBundle(certs.Context(), certificatesmanagement.CreateCaBundleRequest{CreateCaBundleDetails: details})
	if err != nil {
		return certs.ErrorResult(auth.OCIError(err)), nil
	}
	bundle := certs.SummariseCaBundle(&resp.CaBundle)
	return certs.Result(fmt.Sprintf("Created CA bundle %q (%s)", bundle["name"], bundle["lifecycle_state"]), map[string]interface{}{
		"ca_bundle": bundle, "id": bundle["id"], "lifecycle_state": bundle["lifecycle_state"],
	}), nil
}
