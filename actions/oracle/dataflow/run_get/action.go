// Package oracle_dataflow_run_get fetches a single Data Flow run by OCID, returning its
// application, language, lifecycle state and run duration.
package oracle_dataflow_run_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: Get Run"
	Description  = "Fetch a single Data Flow run by its OCID — its application, language, lifecycle state and duration."
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
	{Name: "run_ocid", Type: core.ConnectionTypeString, Label: "Run OCID", Placeholder: "ocid1.dataflowrun.oc1..aaaa…", Required: true},
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
	runID, err := df.RequiredString("run_ocid", inputs)
	if err != nil {
		return df.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetRun(df.Context(), dataflow.GetRunRequest{RunId: &runID})
	if err != nil {
		return df.ErrorResult(auth.OCIError(err)), nil
	}
	run := df.SummariseRun(&resp.Run)
	return df.Result(fmt.Sprintf("Run %q (%s)", run["display_name"], run["lifecycle_state"]), map[string]interface{}{
		"run": run, "id": run["id"], "lifecycle_state": run["lifecycle_state"],
	}), nil
}
