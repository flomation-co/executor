// Package oracle_certificates_certificate_authority_create creates a private root certificate
// authority (CA) whose signing key is generated internally by OCI and protected by a Vault (KMS)
// key. The CA can then issue certificates within your compartment.
package oracle_certificates_certificate_authority_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: Create Certificate Authority"
	Description  = "Create a private root certificate authority (CA) — OCI generates the signing key internally and protects it with the Vault (KMS) key you provide."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "CA Name", Placeholder: "A name for the certificate authority (unique in the compartment)", Required: true},
	{Name: "common_name", Type: core.ConnectionTypeString, Label: "Subject Common Name", Placeholder: "e.g. My Internal Root CA", Required: true},
	{Name: "kms_key_ocid", Type: core.ConnectionTypeString, Label: "Vault (KMS) Key OCID", Placeholder: "ocid1.key.oc1..aaaa… used to protect the CA signing key", Required: true},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "certificate_authority", Type: core.ConnectionTypeObject, Label: "Certificate Authority"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "CA OCID"},
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
	commonName, err := certs.RequiredString("common_name", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}
	kmsKeyID, err := certs.RequiredString("kms_key_ocid", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}

	config := certificatesmanagement.CreateRootCaByGeneratingInternallyConfigDetails{
		Subject: &certificatesmanagement.CertificateSubject{CommonName: &commonName},
	}

	details := certificatesmanagement.CreateCertificateAuthorityDetails{
		Name:                       &name,
		CompartmentId:              &compartment,
		CertificateAuthorityConfig: config,
		KmsKeyId:                   &kmsKeyID,
	}
	if d := certs.OptionalString("description", inputs); d != "" {
		details.Description = &d
	}

	resp, err := client.CreateCertificateAuthority(certs.Context(), certificatesmanagement.CreateCertificateAuthorityRequest{
		CreateCertificateAuthorityDetails: details,
	})
	if err != nil {
		return certs.ErrorResult(auth.OCIError(err)), nil
	}

	ca := certs.SummariseCertificateAuthority(&resp.CertificateAuthority)
	return certs.Result(fmt.Sprintf("Created certificate authority %q (%s)", ca["name"], ca["lifecycle_state"]), map[string]interface{}{
		"certificate_authority": ca,
		"id":                    ca["id"],
		"lifecycle_state":       ca["lifecycle_state"],
	}), nil
}
