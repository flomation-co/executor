// Package oracle_language_project_change_compartment moves an OCI Language project from one
// compartment to another. The project keeps its OCID; only its compartment placement (for access
// control and billing) changes.
package oracle_language_project_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	lang "flomation.app/automate/executor/actions/oracle/language"

	"github.com/oracle/oci-go-sdk/v65/ailanguage"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Language: Change Project Compartment"
	Description  = "Move a Language project into a different compartment — the project keeps its OCID, only its compartment placement changes."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+comments"
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
	{Name: "project_ocid", Type: core.ConnectionTypeString, Label: "Project OCID", Placeholder: "ocid1.ailanguageproject.oc1..aaaa… (the project to move)", Required: true},
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
	auth, client, errResult := lang.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	projectID, err := lang.RequiredString("project_ocid", inputs)
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}
	destination, err := lang.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return lang.ErrorResult(err.Error()), nil
	}

	_, err = client.ChangeProjectCompartment(lang.Context(), ailanguage.ChangeProjectCompartmentRequest{
		ProjectId: &projectID,
		ChangeProjectCompartmentDetails: ailanguage.ChangeProjectCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return lang.ErrorResult(auth.OCIError(err)), nil
	}

	return lang.Result(fmt.Sprintf("Moved project %s into compartment %s", projectID, destination), map[string]interface{}{
		"id":                         projectID,
		"destination_compartment_id": destination,
	}), nil
}
