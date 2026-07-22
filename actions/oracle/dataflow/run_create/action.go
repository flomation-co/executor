// Package oracle_dataflow_run_create launches a run of a Data Flow application — one execution of the
// application's Spark job in the given compartment. Synchronous create: returns the Run, which starts
// in ACCEPTED and progresses through IN_PROGRESS to SUCCEEDED/FAILED (poll Get Run to follow it).
package oracle_dataflow_run_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: Create Run"
	Description  = "Launch a run of a Data Flow application — one execution of its Spark job. Optionally override the display name. Returns the run, which starts in ACCEPTED; poll Get Run until it reaches SUCCEEDED or FAILED."
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
	{Name: "application_ocid", Type: core.ConnectionTypeString, Label: "Application OCID", Placeholder: "ocid1.dataflowapplication.oc1..aaaa… — the application to run", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the run (defaults to the application's, optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "run", Type: core.ConnectionTypeObject, Label: "Run"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Run OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := df.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}
	applicationID, err := df.RequiredString("application_ocid", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}

	details := dataflow.CreateRunDetails{
		CompartmentId: &compartment,
		ApplicationId: &applicationID,
	}
	if name := df.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}

	resp, err := client.CreateRun(df.Context(), dataflow.CreateRunRequest{CreateRunDetails: details})
	if err != nil {
		return df.ErrorResult(auth.OCIError(err)), nil
	}
	run := df.SummariseRun(&resp.Run)
	return df.Result(fmt.Sprintf("Started run %q (%s)", run["display_name"], run["lifecycle_state"]), map[string]interface{}{
		"run": run, "id": run["id"], "lifecycle_state": run["lifecycle_state"],
	}), nil
}
