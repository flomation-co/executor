// Package oracle_exadata_maintenance_run_get reads one Exadata maintenance run by
// OCID — its type, target resource, schedule and lifecycle state.
package oracle_exadata_maintenance_run_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Get Maintenance Run"
	Description  = "Fetch a single Exadata maintenance run by OCID — its maintenance type, target resource, schedule and lifecycle state."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microchip"
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
	{Name: "maintenance_run_ocid", Type: core.ConnectionTypeString, Label: "Maintenance Run OCID", Placeholder: "ocid1.maintenancerun.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "maintenance_run", Type: core.ConnectionTypeObject, Label: "Maintenance Run"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Maintenance Run OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := exa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := exa.RequiredString("maintenance_run_ocid", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetMaintenanceRun(exa.Context(), db.GetMaintenanceRunRequest{MaintenanceRunId: &id})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	run := exa.SummariseMaintenanceRun(&resp.MaintenanceRun)
	return exa.Result(fmt.Sprintf("Maintenance run %q is %s", run["display_name"], run["lifecycle_state"]), map[string]interface{}{
		"maintenance_run": run, "id": run["id"], "lifecycle_state": run["lifecycle_state"],
	}), nil
}
