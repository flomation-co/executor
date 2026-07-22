// Package oracle_certificates_certificate_update applies a partial update to a Certificate: only
// the description you supply is changed; a blank description leaves the existing one unchanged.
package oracle_certificates_certificate_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: Update Certificate"
	Description  = "Partially update a certificate — change only the description you supply; a blank description leaves the certificate unchanged."
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
	{Name: "certificate_ocid", Type: core.ConnectionTypeString, Label: "Certificate OCID", Placeholder: "ocid1.certificate.oc1..aaaa… — the certificate to update", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "certificate", Type: core.ConnectionTypeObject, Label: "Certificate"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Certificate OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := certs.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	certID, err := certs.RequiredString("certificate_ocid", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the description when the operator actually supplied one, so a blank
	// field leaves the certificate's existing description unchanged.
	details := certificatesmanagement.UpdateCertificateDetails{}
	if v := certs.OptionalString("description", inputs); v != "" {
		details.Description = &v
	}

	resp, err := client.UpdateCertificate(certs.Context(), certificatesmanagement.UpdateCertificateRequest{
		CertificateId:            &certID,
		UpdateCertificateDetails: details,
	})
	if err != nil {
		return certs.ErrorResult(auth.OCIError(err)), nil
	}
	certificate := certs.SummariseCertificate(&resp.Certificate)
	return certs.Result(fmt.Sprintf("Updated certificate %q (%s)", certificate["name"], certificate["lifecycle_state"]), map[string]interface{}{
		"certificate": certificate,
		"id":          certificate["id"],
	}), nil
}
