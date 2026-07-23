// Package oracle_certificates_association_list lists the associations between certificate-related
// resources (certificates, CAs, CA bundles) and the Oracle Cloud resources that consume them,
// optionally filtered to a single certificate resource. Walks pagination up to a safe cap.
package oracle_certificates_association_list

import (
	"fmt"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: List Associations"
	Description  = "List the associations between certificate-related resources and the Oracle Cloud resources that use them. Optionally filter to a single certificate resource. Walks pagination up to a safe cap."
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
	{Name: "certificate_ocid", Type: core.ConnectionTypeString, Label: "Certificate Resource OCID Filter", Placeholder: "Only associations for this certificate/CA/CA-bundle OCID (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "associations", Type: core.ConnectionTypeObject, Label: "Associations"},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return certs.ErrorResult(err.Error()), nil
	}
	req := certificatesmanagement.ListAssociationsRequest{CompartmentId: &compartment}
	if certResource := certs.OptionalString("certificate_ocid", inputs); certResource != "" {
		req.CertificatesResourceId = &certResource
	}
	var out []map[string]interface{}
	truncated := false
	for page := 0; ; page++ {
		if page >= certs.ListMaxPages {
			truncated = true
			break
		}
		resp, err := client.ListAssociations(certs.Context(), req)
		if err != nil {
			return certs.ErrorResult(auth.OCIError(err)), nil
		}
		for i := range resp.Items {
			a := &resp.Items[i]
			out = append(out, map[string]interface{}{
				"id":                       certs.Str(a.Id),
				"name":                     certs.Str(a.Name),
				"association_type":         string(a.AssociationType),
				"certificates_resource_id": certs.Str(a.CertificatesResourceId),
				"associated_resource_id":   certs.Str(a.AssociatedResourceId),
				"compartment_id":           certs.Str(a.CompartmentId),
				"lifecycle_state":          string(a.LifecycleState),
				"time_created":             certs.FormatTime(a.TimeCreated),
			})
		}
		if resp.OpcNextPage == nil || *resp.OpcNextPage == "" {
			break
		}
		req.Page = resp.OpcNextPage
	}
	return certs.Result(fmt.Sprintf("Found %d association(s)", len(out)), map[string]interface{}{
		"associations": out, "count": fmt.Sprintf("%d", len(out)), "truncated": truncated,
	}), nil
}
