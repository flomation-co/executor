// Package oracle_certificates_certificate_bundle_get reads an issued certificate bundle from the
// certificates data plane by certificate OCID — the certificate PEM, its chain, serial and version,
// plus the private key PEM when OCI manages it. An optional version number pins a specific version.
package oracle_certificates_certificate_bundle_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificates"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: Get Certificate Bundle"
	Description  = "Read an issued certificate bundle by its OCID — certificate PEM, chain, serial, version and the private key when OCI manages it."
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
	{Name: "certificate_ocid", Type: core.ConnectionTypeString, Label: "Certificate OCID", Placeholder: "ocid1.certificate.oc1..aaaa…", Required: true},
	{Name: "version_number", Type: core.ConnectionTypeString, Label: "Version Number", Placeholder: "Optional — a specific certificate version (defaults to the current version)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "bundle", Type: core.ConnectionTypeObject, Label: "Certificate Bundle"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := certs.DataClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	certID, err := certs.RequiredString("certificate_ocid", inputs)
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}

	// Ask for the private key too; when OCI manages it the response comes back as a
	// CertificateBundleWithPrivateKey, otherwise it's public-only and private_key_pem is omitted.
	req := certificates.GetCertificateBundleRequest{
		CertificateId:         &certID,
		CertificateBundleType: certificates.GetCertificateBundleCertificateBundleTypeWithPrivateKey,
	}
	if vn, ok, err := certs.OptionalInt("version_number", inputs); err != nil {
		return certs.ErrorResult(err.Error()), nil
	} else if ok {
		v := int64(vn)
		req.VersionNumber = &v
	}

	resp, err := client.GetCertificateBundle(certs.Context(), req)
	if err != nil {
		return certs.ErrorResult(auth.OCIError(err)), nil
	}

	bundle := map[string]interface{}{
		"certificate_id":   certs.Str(resp.GetCertificateId()),
		"certificate_name": certs.Str(resp.GetCertificateName()),
		"version_number":   certs.Int64OrNil(resp.GetVersionNumber()),
		"version_name":     certs.Str(resp.GetVersionName()),
		"serial":           certs.Str(resp.GetSerialNumber()),
		"certificate_pem":  certs.Str(resp.GetCertificatePem()),
		"cert_chain_pem":   certs.Str(resp.GetCertChainPem()),
		"time_created":     certs.FormatTime(resp.GetTimeCreated()),
	}
	if wpk, ok := resp.CertificateBundle.(certificates.CertificateBundleWithPrivateKey); ok {
		if pk := certs.Str(wpk.PrivateKeyPem); pk != "" {
			bundle["private_key_pem"] = pk
		}
	}

	return certs.Result(fmt.Sprintf("Fetched certificate bundle for %q (version %v)", bundle["certificate_name"], bundle["version_number"]), map[string]interface{}{
		"bundle": bundle,
	}), nil
}
