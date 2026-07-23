// Package oracle_certificates_certificate_authority_schedule_deletion schedules a certificate
// authority (CA) for deletion, optionally at a future time.
package oracle_certificates_certificate_authority_schedule_deletion

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	certs "flomation.app/automate/executor/actions/oracle/certificates"

	"github.com/oracle/oci-go-sdk/v65/certificatesmanagement"
	"github.com/oracle/oci-go-sdk/v65/common"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Certificates: Schedule CA Deletion"
	Description  = "Schedule a certificate authority (CA) for deletion, optionally at a future time."
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
	{Name: "certificate_authority_ocid", Type: core.ConnectionTypeString, Label: "Certificate Authority OCID", Placeholder: "ocid1.certificateauthority.oc1..aaaa… of the CA to schedule for deletion", Required: true},
	{Name: "time_of_deletion", Type: core.ConnectionTypeString, Label: "Time of Deletion", Placeholder: "Optional — when to delete, RFC 3339 (e.g. 2026-12-31T00:00:00Z)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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

	details := certificatesmanagement.ScheduleCertificateAuthorityDeletionDetails{}
	if v := strings.TrimSpace(certs.OptionalString("time_of_deletion", inputs)); v != "" {
		t, perr := time.Parse(time.RFC3339, v)
		if perr != nil {
			return certs.ErrorResult(fmt.Sprintf("invalid time of deletion %q: expected RFC3339 (e.g. 2026-12-31T00:00:00Z)", v)), nil
		}
		details.TimeOfDeletion = &common.SDKTime{Time: t}
	}

	_, err = client.ScheduleCertificateAuthorityDeletion(certs.Context(), certificatesmanagement.ScheduleCertificateAuthorityDeletionRequest{
		CertificateAuthorityId:                      &caID,
		ScheduleCertificateAuthorityDeletionDetails: details,
	})
	if err != nil {
		return certs.ErrorResult(auth.OCIError(err)), nil
	}
	return certs.Result(fmt.Sprintf("Scheduled certificate authority %s for deletion", caID), map[string]interface{}{
		"id": caID,
	}), nil
}
