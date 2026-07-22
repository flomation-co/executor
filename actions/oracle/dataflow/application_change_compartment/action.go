// Package oracle_dataflow_application_change_compartment moves a Data Flow application from one
// compartment to another. The application keeps its OCID; only its compartment placement (for
// access control and billing) changes.
package oracle_dataflow_application_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: Change Application Compartment"
	Description  = "Move a Data Flow application into a different compartment — the application keeps its OCID, only its compartment placement changes."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+diagram-project"
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
	{Name: "application_ocid", Type: core.ConnectionTypeString, Label: "Application OCID", Placeholder: "ocid1.dataflowapplication.oc1..aaaa… (the application to move)", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (where to move the application)", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Application OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := df.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	applicationID, err := df.RequiredString("application_ocid", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	destination, err := df.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}

	_, err = client.ChangeApplicationCompartment(df.Context(), dataflow.ChangeApplicationCompartmentRequest{
		ApplicationId: &applicationID,
		ChangeApplicationCompartmentDetails: dataflow.ChangeApplicationCompartmentDetails{
			CompartmentId: &destination,
		},
	})
	if err != nil {
		return df.ErrorResult(auth.OCIError(err)), nil
	}

	return df.Result(fmt.Sprintf("Moved application %s into compartment %s", applicationID, destination), map[string]interface{}{
		"id":                         applicationID,
		"destination_compartment_id": destination,
	}), nil
}
