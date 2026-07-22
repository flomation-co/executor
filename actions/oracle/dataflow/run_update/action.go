// Package oracle_dataflow_run_update applies a partial update to a Data Flow run: only the run
// properties OCI actually lets you change after launch — the maximum duration, the SESSION idle
// timeout and the free-form tags — are touched; blank fields are left unchanged.
package oracle_dataflow_run_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	df "flomation.app/automate/executor/actions/oracle/dataflow"

	"github.com/oracle/oci-go-sdk/v65/dataflow"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Data Flow: Update Run"
	Description  = "Partially update a Data Flow run — change only the maximum duration, SESSION idle timeout or free-form tags you supply; blank fields are left unchanged (a run's display name and other properties are immutable once launched)."
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
	{Name: "run_ocid", Type: core.ConnectionTypeString, Label: "Run OCID", Placeholder: "ocid1.dataflowrun.oc1..aaaa… — the run to update", Required: true},
	{Name: "max_duration_in_minutes", Type: core.ConnectionTypeString, Label: "Max Duration (minutes)", Placeholder: "Terminate the run after this many minutes IN_PROGRESS (leave blank to keep unchanged)"},
	{Name: "idle_timeout_in_minutes", Type: core.ConnectionTypeString, Label: "Idle Timeout (minutes)", Placeholder: "SESSION runs only — stop after this much inactivity (leave blank to keep unchanged)"},
	{Name: "freeform_tags", Type: core.ConnectionTypeText, Label: "Free-form Tags (JSON)", Placeholder: "{\"Department\":\"Finance\"} — replaces existing tags (leave blank to keep unchanged)"},
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

	// Partial update: only carry the fields the operator actually supplied. Blank fields stay nil so
	// the run keeps its existing value.
	details := dataflow.UpdateRunDetails{}
	if n, ok, err := df.OptionalInt("max_duration_in_minutes", inputs); err != nil {
		return df.ErrorResult(err.Error()), nil
	} else if ok {
		v := int64(n)
		details.MaxDurationInMinutes = &v
	}
	if n, ok, err := df.OptionalInt("idle_timeout_in_minutes", inputs); err != nil {
		return df.ErrorResult(err.Error()), nil
	} else if ok {
		v := int64(n)
		details.IdleTimeoutInMinutes = &v
	}
	if tags, err := df.FreeformTags("freeform_tags", inputs); err != nil {
		return df.ErrorResult(err.Error()), nil
	} else if tags != nil {
		details.FreeformTags = tags
	}

	resp, err := client.UpdateRun(df.Context(), dataflow.UpdateRunRequest{RunId: &runID, UpdateRunDetails: details})
	if err != nil {
		return df.ErrorResult(auth.OCIError(err)), nil
	}
	run := df.SummariseRun(&resp.Run)
	return df.Result(fmt.Sprintf("Updated run %q (%s)", run["display_name"], run["lifecycle_state"]), map[string]interface{}{
		"run": run, "id": run["id"], "lifecycle_state": run["lifecycle_state"],
	}), nil
}
