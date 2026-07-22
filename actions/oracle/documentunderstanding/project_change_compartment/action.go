// Package oracle_documentunderstanding_project_change_compartment moves an OCI Document
// Understanding project from one compartment to another. The project keeps its OCID; only its
// compartment placement (for access control and billing) changes.
package oracle_documentunderstanding_project_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	du "flomation.app/automate/executor/actions/oracle/documentunderstanding"

	"github.com/oracle/oci-go-sdk/v65/aidocument"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Document Understanding: Change Project Compartment"
	Description  = "Move a Document Understanding project into a different compartment — the project keeps its OCID, only its compartment placement changes."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+image"
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
	{Name: "project_ocid", Type: core.ConnectionTypeString, Label: "Project OCID", Placeholder: "ocid1.aidocumentproject.oc1..aaaa… (the project to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the project)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Project OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := du.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	projectID, err := du.RequiredString("project_ocid", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}
	destination, err := du.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return du.ErrorResult(err.Error()), nil
	}

	_, err = client.ChangeProjectCompartment(du.Context(), aidocument.ChangeProjectCompartmentRequest{
		ProjectId: &projectID,
		ChangeProjectCompartmentDetails: aidocument.ChangeProjectCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return du.ErrorResult(auth.OCIError(err)), nil
	}

	return du.Result(fmt.Sprintf("Moved project %s into compartment %s", projectID, destination), map[string]interface{}{
		"id":                         projectID,
		"destination_compartment_id": destination,
	}), nil
}
