// Package oracle_autonomousdatabase_db_change_compartment moves an Oracle Cloud
// Autonomous Database to a different compartment. The operation is asynchronous —
// it returns immediately with a work-request id.
package oracle_autonomousdatabase_db_change_compartment

import (
	"fmt"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: Move to Compartment"
	Description  = "Move an Oracle Cloud Autonomous Database to a different compartment. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+folder"
	Date         = "21/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Required: true},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Required: true},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Required: true},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Required: true},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines"},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)"},
	{Name: "autonomous_database_id", Type: core.ConnectionTypeString, Label: "Autonomous Database OCID", Placeholder: "ocid1.autonomousdatabase.oc1..aaaa…", Required: true},
	{Name: "target_compartment_ocid", Type: core.ConnectionTypeString, Label: "Target Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — the compartment to move the database into", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := adb.PerDatabaseClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	target, err := adb.RequiredString("target_compartment_ocid", inputs)
	if err != nil {
		return adb.ErrorResult("target compartment OCID is required (the compartment to move the database into)"), nil
	}
	resp, err := client.ChangeAutonomousDatabaseCompartment(adb.Context(), database.ChangeAutonomousDatabaseCompartmentRequest{
		AutonomousDatabaseId:     &id,
		ChangeCompartmentDetails: database.ChangeCompartmentDetails{CompartmentId: &target},
	})
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Move requested for Autonomous Database %q to compartment %s", id, target),
		"work_request_id": adb.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
