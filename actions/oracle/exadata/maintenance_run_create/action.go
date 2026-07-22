// Package oracle_exadata_maintenance_run_create schedules a maintenance run for an Exadata
// resource — the patching window in which Oracle applies the latest Release Update. Pick the
// target resource, the scheduled time, and (optionally) whether patching is rolling or
// non-rolling. Returns the newly created maintenance run.
package oracle_exadata_maintenance_run_create

import (
	"fmt"
	"strings"
	"time"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	"github.com/oracle/oci-go-sdk/v65/common"
	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Create Maintenance Run"
	Description  = "Schedule a maintenance run for an Exadata resource — the window in which Oracle applies the latest Release Update. Give it the target resource OCID, an RFC3339 scheduled time, and optionally a rolling or non-rolling patching mode."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… for the maintenance run (optional)"},
	{Name: "target_resource_ocid", Type: core.ConnectionTypeString, Label: "Target Resource OCID", Placeholder: "ocid1..aaaa… of the resource to maintain (VM cluster, infrastructure, …)", Required: true},
	{Name: "patch_type", Type: core.ConnectionTypeString, Label: "Patch Type", Placeholder: "What kind of maintenance to schedule", Required: true, Options: []core.ConnectionOption{
		{Name: "Quarterly (infrastructure RU)", Value: "QUARTERLY"},
		{Name: "Time zone", Value: "TIMEZONE"},
		{Name: "Custom database software image", Value: "CUSTOM_DATABASE_SOFTWARE_IMAGE"},
	}},
	{Name: "time_scheduled", Type: core.ConnectionTypeString, Label: "Time Scheduled", Placeholder: "RFC3339, e.g. 2026-08-01T02:00:00Z", Required: true},
	{Name: "patching_mode", Type: core.ConnectionTypeString, Label: "Patching Mode", Placeholder: "Rolling or non-rolling (optional)", Options: []core.ConnectionOption{
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
	targetID, err := exa.RequiredString("target_resource_ocid", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	scheduledRaw, err := exa.RequiredString("time_scheduled", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	t, err := time.Parse(time.RFC3339, scheduledRaw)
	if err != nil {
		return exa.ErrorResult("time scheduled must be an RFC3339 timestamp, e.g. 2026-08-01T02:00:00Z"), nil
	}
	patchType := strings.ToUpper(strings.TrimSpace(exa.OptionalString("patch_type", inputs)))
	switch patchType {
	case "QUARTERLY", "TIMEZONE", "CUSTOM_DATABASE_SOFTWARE_IMAGE":
	default:
		return exa.ErrorResult("patch type must be QUARTERLY, TIMEZONE or CUSTOM_DATABASE_SOFTWARE_IMAGE"), nil
	}
	sdkTime := common.SDKTime{Time: t}
	details := db.CreateMaintenanceRunDetails{
		TargetResourceId: &targetID,
		TimeScheduled:    &sdkTime,
		PatchType:        db.CreateMaintenanceRunDetailsPatchTypeEnum(patchType),
	}
	if pm := exa.OptionalString("patching_mode", inputs); pm != "" {
		details.PatchingMode = db.CreateMaintenanceRunDetailsPatchingModeEnum(pm)
	}
	if compartment := exa.OptionalString("compartment_ocid", inputs); compartment != "" {
		details.CompartmentId = &compartment
	}
	resp, err := client.CreateMaintenanceRun(exa.Context(), db.CreateMaintenanceRunRequest{CreateMaintenanceRunDetails: details})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	run := exa.SummariseMaintenanceRun(&resp.MaintenanceRun)
	return exa.Result(fmt.Sprintf("Scheduled maintenance run %q (%s) for target %s", run["id"], run["lifecycle_state"], targetID), map[string]interface{}{
		"maintenance_run": run, "id": run["id"], "lifecycle_state": run["lifecycle_state"],
	}), nil
}
