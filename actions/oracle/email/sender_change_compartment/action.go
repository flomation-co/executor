// Package oracle_email_sender_change_compartment moves an OCI Email Delivery approved sender from
// one compartment to another. The sender keeps its OCID; only its compartment placement (for
// access control and billing) changes.
package oracle_email_sender_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	em "flomation.app/automate/executor/actions/oracle/email"

	"github.com/oracle/oci-go-sdk/v65/email"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Email: Change Sender Compartment"
	Description  = "Move an approved sender into a different compartment — the sender keeps its OCID, only its compartment placement changes."
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
	{Name: "sender_ocid", Type: core.ConnectionTypeString, Label: "Sender OCID", Placeholder: "ocid1.emailsender.oc1..aaaa… (the sender to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the sender)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Sender OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := em.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	senderID, err := em.RequiredString("sender_ocid", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}
	destination, err := em.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return em.ErrorResult(err.Error()), nil
	}

	_, err = client.ChangeSenderCompartment(em.Context(), email.ChangeSenderCompartmentRequest{
		SenderId: &senderID,
		ChangeSenderCompartmentDetails: email.ChangeSenderCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return em.ErrorResult(auth.OCIError(err)), nil
	}

	return em.Result(fmt.Sprintf("Moved sender %s into compartment %s", senderID, destination), map[string]interface{}{
		"id":                         senderID,
		"destination_compartment_id": destination,
	}), nil
}
