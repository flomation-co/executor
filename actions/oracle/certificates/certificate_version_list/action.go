// Package oracle_certificates_certificate_version_list lists the versions of a certificate,
// returning each version's number, serial number, rotation stages and creation time. Walks
// pagination up to a safe cap.
package oracle_certificates_certificate_version_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: List Certificate Versions"
	Description  = "List the versions of a certificate — each version's number, serial number, rotation stages and creation time. Walks pagination up to a safe cap."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "certificate_ocid", Type: core.ConnectionTypeString, Label: "Certificate OCID", Placeholder: "ocid1.certificate.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "versions", Type: core.ConnectionTypeObject, Label: "Versions"},
	{Name: "count", Type: core.ConnectionTypeString, Label: "Count"},
	{Name: "truncated", Type: core.ConnectionTypeBoolean, Label: "Truncated"},
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

	req := certificatesmanagement.ListCertificateVersionsRequest{CertificateId: &certID}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= certs.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListCertificateVersions(certs.Context(), req)
		if err != nil {
			return certs.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			out = append(out, summariseVersion(&resp.Items[i]))
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return certs.Result(fmt.Sprintf("Found %d certificate version(s)", len(out)), map[string]interface{}{
		"versions": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}

func summariseVersion(v *certificatesmanagement.CertificateVersionSummary) map[string]interface{} {
	stages := make([]string, 0, len(v.Stages))
	for _, s := range v.Stages {
		stages = append(stages, string(s))
	}
	return map[string]interface{}{
		"version_number": certs.Int64OrNil(v.VersionNumber),
		"version_name":   certs.Str(v.VersionName),
		"serial_number":  certs.Str(v.SerialNumber),
		"stages":         stages,
		"time_created":   certs.FormatTime(v.TimeCreated),
	}
}
