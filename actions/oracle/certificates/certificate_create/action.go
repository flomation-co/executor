// Package oracle_certificates_certificate_create creates an OCI-managed certificate that is
// issued by a private certificate authority (CA) you already run in the Certificates service.
// The certificate's contents (common name, key/signature algorithms, profile) are described by
// the issued-by-internal-CA config; imported and externally-managed configs are out of scope.
package oracle_certificates_certificate_create

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: Create Certificate"
	Description  = "Create a certificate issued by one of your private certificate authorities (CAs) — set the common name, profile and key/signature algorithms."
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
	{Name: "name", Type: core.ConnectionTypeString, Label: "Certificate Name", Placeholder: "Unique within the compartment", Required: true},
	{Name: "issuer_certificate_authority_id", Type: core.ConnectionTypeString, Label: "Issuer CA OCID", Placeholder: "ocid1.certificateauthority.oc1..aaaa… (the private CA that signs it)", Required: true},
	{Name: "common_name", Type: core.ConnectionTypeString, Label: "Common Name", Placeholder: "e.g. www.example.com — the certificate subject CN", Required: true},
	{Name: "certificate_profile_type", Type: core.ConnectionTypeString, Label: "Certificate Profile", Placeholder: "How the certificate will be used", Required: true, Options: []core.ConnectionOption{
		{Name: "TLS server or client", Value: "TLS_SERVER_OR_CLIENT"},
		{Name: "TLS server", Value: "TLS_SERVER"},
		{Name: "TLS client", Value: "TLS_CLIENT"},
		{Name: "TLS code sign", Value: "TLS_CODE_SIGN"},
	}},
	{Name: "subject_alternative_dns_names", Type: core.ConnectionTypeString, Label: "Subject Alternative DNS Names", Placeholder: "Comma-separated DNS names, e.g. example.com, api.example.com (optional)"},
	{Name: "key_algorithm", Type: core.ConnectionTypeString, Label: "Key Algorithm", Placeholder: "Defaults to the service default (optional)", Options: []core.ConnectionOption{
		{Name: "RSA 2048", Value: "RSA2048"},
		{Name: "RSA 4096", Value: "RSA4096"},
		{Name: "ECDSA P-256", Value: "ECDSA_P256"},
		{Name: "ECDSA P-384", Value: "ECDSA_P384"},
	}},
	{Name: "signature_algorithm", Type: core.ConnectionTypeString, Label: "Signature Algorithm", Placeholder: "Defaults to the service default (optional)", Options: []core.ConnectionOption{
		{Name: "SHA-256 with RSA", Value: "SHA256_WITH_RSA"},
		{Name: "SHA-384 with RSA", Value: "SHA384_WITH_RSA"},
		{Name: "SHA-512 with RSA", Value: "SHA512_WITH_RSA"},
		{Name: "SHA-256 with ECDSA", Value: "SHA256_WITH_ECDSA"},
		{Name: "SHA-384 with ECDSA", Value: "SHA384_WITH_ECDSA"},
		{Name: "SHA-512 with ECDSA", Value: "SHA512_WITH_ECDSA"},
	}},
	{Name: "version_name", Type: core.ConnectionTypeString, Label: "Version Name", Placeholder: "A name for this first version (optional)"},
	{Name: "description", Type: core.ConnectionTypeString, Label: "Description", Placeholder: "Optional"},
	{Name: "freeform_tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: "{\"env\":\"prod\"} (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "certificate", Type: core.ConnectionTypeObject, Label: "Certificate"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Certificate OCID"},
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
	issuerCA, err := certs.RequiredString("issuer_certificate_authority_id", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}
	commonName, err := certs.RequiredString("common_name", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}
	profileType, err := certs.RequiredString("certificate_profile_type", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}

	config := certificatesmanagement.CreateCertificateIssuedByInternalCaConfigDetails{
		IssuerCertificateAuthorityId: &issuerCA,
		Subject:                      &certificatesmanagement.CertificateSubject{CommonName: &commonName},
		CertificateProfileType:       certificatesmanagement.CertificateProfileTypeEnum(profileType),
	}
	if v := certs.OptionalString("version_name", inputs); strings.TrimSpace(v) != "" {
		vn := strings.TrimSpace(v)
		config.VersionName = &vn
	}
	if v := strings.TrimSpace(certs.OptionalString("key_algorithm", inputs)); v != "" {
		config.KeyAlgorithm = certificatesmanagement.KeyAlgorithmEnum(v)
	}
	if v := strings.TrimSpace(certs.OptionalString("signature_algorithm", inputs)); v != "" {
		config.SignatureAlgorithm = certificatesmanagement.SignatureAlgorithmEnum(v)
	}
	if sans := parseSANs(certs.OptionalString("subject_alternative_dns_names", inputs)); len(sans) > 0 {
		config.SubjectAlternativeNames = sans
	}

	details := certificatesmanagement.CreateCertificateDetails{
		Name:              &name,
		CompartmentId:     &compartment,
		CertificateConfig: config,
	}
	if d := strings.TrimSpace(certs.OptionalString("description", inputs)); d != "" {
		details.Description = &d
	}
	if tags, err := certs.FreeformTags("freeform_tags", inputs); err != nil {
		return certs.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.CreateCertificate(certs.Context(), certificatesmanagement.CreateCertificateRequest{CreateCertificateDetails: details})
	if err != nil {
		return certs.ErrorResult(auth.OCIError(err)), nil
	}
	cert := certs.SummariseCertificate(&resp.Certificate)
	return certs.Result(fmt.Sprintf("Created certificate %q (%s)", cert["name"], cert["lifecycle_state"]), map[string]interface{}{
		"certificate":     cert,
		"id":              cert["id"],
		"lifecycle_state": cert["lifecycle_state"],
	}), nil
}

// parseSANs turns a comma-separated list of DNS names into DNS-typed subject alternative names,
// skipping blanks so a trailing comma or double comma never produces an empty (invalid) entry.
func parseSANs(raw string) []certificatesmanagement.CertificateSubjectAlternativeName {
	var out []certificatesmanagement.CertificateSubjectAlternativeName
	for _, part := range strings.Split(raw, ",") {
		v := strings.TrimSpace(part)
		if v == "" {
			continue
		}
		val := v
		out = append(out, certificatesmanagement.CertificateSubjectAlternativeName{
			Type:  certificatesmanagement.CertificateSubjectAlternativeNameTypeDns,
			Value: &val,
		})
	}
	return out
}
