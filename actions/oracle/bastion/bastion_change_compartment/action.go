// Package oracle_bastion_bastion_change_compartment moves a bastion from one compartment to
// another. The bastion keeps its OCID; only its compartment placement (for access control and
// billing) changes. The call is accepted asynchronously — OCI returns an opc-request-id you can
// quote to support; poll Get Bastion until the move settles.
package oracle_bastion_bastion_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	bas "flomation.app/automate/executor/actions/oracle/bastion"

	"github.com/oracle/oci-go-sdk/v65/bastion"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Bastion: Change Bastion Compartment"
	Description  = "Move a bastion into a different compartment — the bastion keeps its OCID, only its compartment placement changes."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+terminal"
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
	{Name: "bastion_ocid", Type: core.ConnectionTypeString, Label: "Bastion OCID", Placeholder: "ocid1.bastion.oc1..aaaa… (the bastion to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the bastion)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Bastion OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "opc_request_id", Type: core.ConnectionTypeString, Label: "OCI Request ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := bas.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	bastionID, err := bas.RequiredString("bastion_ocid", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}
	destination, err := bas.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return bas.ErrorResult(err.Error()), nil
	}

	resp, err := client.ChangeBastionCompartment(bas.Context(), bastion.ChangeBastionCompartmentRequest{
		BastionId: &bastionID,
		ChangeBastionCompartmentDetails: bastion.ChangeBastionCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return bas.ErrorResult(auth.OCIError(err)), nil
	}

	return bas.Result(fmt.Sprintf("Moving bastion %s into compartment %s", bastionID, destination), map[string]interface{}{
		"id":                         bastionID,
		"destination_compartment_id": destination,
		"opc_request_id":             bas.Str(resp.OpcRequestId),
	}), nil
}
