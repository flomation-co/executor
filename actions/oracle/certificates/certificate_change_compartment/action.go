// Package oracle_certificates_certificate_change_compartment moves a certificate from one
// compartment to another. The certificate keeps its OCID; only its compartment placement (for
// access control and billing) changes.
package oracle_certificates_certificate_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: Change Certificate Compartment"
	Description  = "Move a certificate into a different compartment — the certificate keeps its OCID, only its compartment placement changes."
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
	{Name: "certificate_ocid", Type: core.ConnectionTypeString, Label: "Certificate OCID", Placeholder: "ocid1.certificate.oc1..aaaa… (the certificate to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the certificate)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Certificate OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := certs.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	certificateID, err := certs.RequiredString("certificate_ocid", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}
	destination, err := certs.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}

	_, err = client.ChangeCertificateCompartment(certs.Context(), certificatesmanagement.ChangeCertificateCompartmentRequest{
		CertificateId: &certificateID,
		ChangeCertificateCompartmentDetails: certificatesmanagement.ChangeCertificateCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return certs.ErrorResult(auth.OCIError(err)), nil
	}

	return certs.Result(fmt.Sprintf("Moved certificate %s into compartment %s", certificateID, destination), map[string]interface{}{
		"id":                         certificateID,
		"destination_compartment_id": destination,
	}), nil
}
