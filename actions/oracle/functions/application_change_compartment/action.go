// Package oracle_functions_application_change_compartment moves a Functions application into a
// different compartment. The application's functions move with it.
package oracle_functions_application_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	fn "flomation.app/automate/executor/actions/oracle/functions"

	"github.com/oracle/oci-go-sdk/v65/functions"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Functions: Move Application"
	Description  = "Move a Functions application (and its functions) into a different compartment."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+code"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "application_ocid", Type: core.ConnectionTypeString, Label: "Application OCID", Placeholder: "ocid1.fnapp.oc1..aaaa…", Required: true},
	{Name: "destination_compartment_ocid", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… to move the application into", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Application OCID"},
	{Name: "destination_compartment_id", Type: core.ConnectionTypeString, Label: "Destination Compartment OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	applicationID, err := fn.RequiredString("application_ocid", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	destination, err := fn.RequiredString("destination_compartment_ocid", inputs)
	if err != nil {
		return fn.ErrorResult(err.Error()), nil
	}
	auth, client, errResult := fn.MgmtClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	req := functions.ChangeApplicationCompartmentRequest{
		ApplicationId: &applicationID,
		ChangeApplicationCompartmentDetails: functions.ChangeApplicationCompartmentDetails{
			CompartmentId: &destination,
		},
	}
	if _, err := client.ChangeApplicationCompartment(fn.Context(), req); err != nil {
		return fn.ErrorResult(auth.OCIError(err)), nil
	}
	return fn.Result(fmt.Sprintf("Moved application %s to compartment %s", applicationID, destination), map[string]interface{}{
		"id":                         applicationID,
		"destination_compartment_id": destination,
	}), nil
}
