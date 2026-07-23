// Package oracle_exadata_maintenance_run_update modifies a scheduled Exadata
// maintenance run — reschedule it, enable or skip it, start patching now, or
// switch its patching mode (rolling vs non-rolling). Synchronous — it returns
// the updated maintenance run.
package oracle_exadata_maintenance_run_update

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	"github.com/oracle/oci-go-sdk/v65/common"
	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Update Maintenance Run"
	Description  = "Modify a scheduled Exadata maintenance run — reschedule it, enable or skip it, start patching now, or switch its patching mode."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microchip"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "maintenance_run_ocid", Type: core.ConnectionTypeString, Label: "Maintenance Run OCID", Placeholder: "ocid1.maintenancerun.oc1..aaaa…", Required: true},
	{Name: "is_enabled", Type: core.ConnectionTypeBoolean, Label: "Enabled", Placeholder: "Set false to skip this maintenance run (optional)"},
	{Name: "time_scheduled", Type: core.ConnectionTypeString, Label: "Scheduled Time", Placeholder: "Reschedule to, RFC3339 e.g. 2026-07-22T14:30:00Z (optional)"},
	{Name: "is_patch_now_enabled", Type: core.ConnectionTypeBoolean, Label: "Patch Now", Placeholder: "Set true to start patching immediately (optional)"},
	{Name: "patching_mode", Type: core.ConnectionTypeString, Label: "Patching Mode", Placeholder: "Infrastructure node patching method (optional)", Options: []core.ConnectionOption{
		{Name: "Rolling", Value: "ROLLING"},
		{Name: "Non-rolling (involves downtime)", Value: "NONROLLING"},
	}},
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

	details := db.UpdateMaintenanceRunDetails{}
	if exa.BoolWasSet("is_enabled", inputs) {
		v := exa.OptionalBool("is_enabled", inputs, false)
		details.IsEnabled = &v
	}
	if v := exa.OptionalString("time_scheduled", inputs); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return exa.ErrorResult(fmt.Sprintf("invalid scheduled time %q: expected RFC3339 (e.g. 2026-07-22T14:30:00Z)", v)), nil
		}
		details.TimeScheduled = &common.SDKTime{Time: t}
	}
	if exa.BoolWasSet("is_patch_now_enabled", inputs) {
		v := exa.OptionalBool("is_patch_now_enabled", inputs, false)
		details.IsPatchNowEnabled = &v
	}
	if pm := exa.OptionalString("patching_mode", inputs); pm != "" {
		details.PatchingMode = db.UpdateMaintenanceRunDetailsPatchingModeEnum(pm)
	}

	resp, err := client.UpdateMaintenanceRun(exa.Context(), db.UpdateMaintenanceRunRequest{
		MaintenanceRunId:            &id,
		UpdateMaintenanceRunDetails: details,
	})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}

	run := exa.SummariseMaintenanceRun(&resp.MaintenanceRun)
	return exa.Result(fmt.Sprintf("Maintenance run %q is %s", run["display_name"], run["lifecycle_state"]), map[string]interface{}{
		"maintenance_run": run, "id": run["id"], "lifecycle_state": run["lifecycle_state"],
	}), nil
}
