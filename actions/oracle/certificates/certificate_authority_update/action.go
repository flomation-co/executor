// Package oracle_certificates_certificate_authority_update applies a partial update to a
// certificate authority (CA): only the description you supply is changed; a blank description is
// left unchanged. The CA's configuration, rules and versions are preserved.
package oracle_certificates_certificate_authority_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: Update Certificate Authority"
	Description  = "Partially update a certificate authority — change only the description you supply; a blank description is left unchanged and the CA's configuration, rules and versions are preserved."
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
	{Name: "certificate_authority_ocid", Type: core.ConnectionTypeString, Label: "Certificate Authority OCID", Placeholder: "ocid1.certificateauthority.oc1..aaaa… — the CA to update", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "New description (leave blank to keep unchanged)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "certificate_authority", Type: core.ConnectionTypeObject, Label: "Certificate Authority"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Certificate Authority OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := certs.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	caID, err := certs.RequiredString("certificate_authority_ocid", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}

	// Partial update: only carry the description when the operator actually supplied one, so a
	// blank field leaves the CA's existing description unchanged.
	details := certificatesmanagement.UpdateCertificateAuthorityDetails{}
	if v := certs.OptionalString("description", inputs); v != "" {
		details.Description = &v
	}

	resp, err := client.UpdateCertificateAuthority(certs.Context(), certificatesmanagement.UpdateCertificateAuthorityRequest{
		CertificateAuthorityId:            &caID,
		UpdateCertificateAuthorityDetails: details,
	})
	if err != nil {
		return certs.ErrorResult(auth.OCIError(err)), nil
	}

	ca := certs.SummariseCertificateAuthority(&resp.CertificateAuthority)
	return certs.Result(fmt.Sprintf("Updated certificate authority %q (%s)", ca["name"], ca["lifecycle_state"]), map[string]interface{}{
		"certificate_authority": ca,
		"id":                    ca["id"],
	}), nil
}
