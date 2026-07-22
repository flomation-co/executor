// Package oracle_email_email_domain_change_compartment moves an Email Delivery sending domain from
// one compartment to another. The domain keeps its OCID; only its compartment placement (for access
// control and billing) changes. Asynchronous: the move returns a work-request id to poll.
package oracle_email_email_domain_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: Change Email Domain Compartment"
	Description  = "Move an email sending domain into a different compartment — the domain keeps its OCID, only its compartment placement changes. Returns a work-request id to poll."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+envelope"
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
	{Name: "email_domain_ocid", Type: core.ConnectionTypeString, Label: "Email Domain OCID", Placeholder: "ocid1.emaildomain.oc1..aaaa… (the domain to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the domain)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Email Domain OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := em.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	domainID, err := em.RequiredString("email_domain_ocid", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}
	destination, err := em.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}

	resp, err := client.ChangeEmailDomainCompartment(em.Context(), email.ChangeEmailDomainCompartmentRequest{
		EmailDomainId: &domainID,
		ChangeEmailDomainCompartmentDetails: email.ChangeEmailDomainCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return em.ErrorResult(auth.OCIError(err)), nil
	}

	return em.Result(fmt.Sprintf("Moving email domain %s into compartment %s", domainID, destination), map[string]interface{}{
		"id":              domainID,
		"work_request_id": em.Str(resp.OpcWorkRequestId),
	}), nil
}
