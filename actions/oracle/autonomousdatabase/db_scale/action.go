// Package oracle_autonomousdatabase_db_scale scales an Oracle Cloud Autonomous
// Database — changing its CPU count, storage, or auto-scaling settings. The
// operation is asynchronous — it returns immediately with a work-request id while
// the database applies the change and transitions back to AVAILABLE.
package oracle_autonomousdatabase_db_scale

import (
	"fmt"
	"strings"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: Scale"
	Description  = "Scale an Oracle Cloud Autonomous Database — change CPU count, storage, or toggle auto-scaling. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+gauge"
	Date         = "21/07/2026"
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
	{Name: "autonomous_database_id", Type: core.ConnectionTypeString, Label: "Autonomous Database OCID", Placeholder: "ocid1.autonomousdatabase.oc1..aaaa…", Required: true},
	{Name: "compute_model", Type: core.ConnectionTypeString, Label: "Compute Model", Placeholder: "Default ECPU", Options: []core.ConnectionOption{
		{Name: "ECPU", Value: "ECPU"},
		{Name: "OCPU", Value: "OCPU"},
	}},
	{Name: "cpu_count", Type: core.ConnectionTypeString, Label: "CPU Count", Placeholder: "New ECPU or OCPU count, e.g. 4 (leave blank to keep current)"},
	{Name: "data_storage_in_tbs", Type: core.ConnectionTypeString, Label: "Storage (TB)", Placeholder: "New data storage in TB, e.g. 2 (leave blank to keep current)"},
	{Name: "is_auto_scaling_enabled", Type: core.ConnectionTypeBoolean, Label: "Compute Auto-scaling", Placeholder: "Toggle compute auto-scaling (leave blank to keep current)"},
	{Name: "is_auto_scaling_for_storage_enabled", Type: core.ConnectionTypeBoolean, Label: "Storage Auto-scaling", Placeholder: "Toggle storage auto-scaling (leave blank to keep current)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "database", Type: core.ConnectionTypeObject, Label: "Database"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := adb.PerDatabaseClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	details := database.UpdateAutonomousDatabaseDetails{}
	model := strings.TrimSpace(adb.OptionalString("compute_model", inputs))
	if model != "" {
		details.ComputeModel = database.UpdateAutonomousDatabaseDetailsComputeModelEnum(model)
	}
	if cnt, ok, err := adb.OptionalFloat32("cpu_count", inputs); err != nil {
		return adb.ErrorResult(err.Error()), nil
	} else if ok {
		if strings.EqualFold(model, "OCPU") {
			details.OcpuCount = &cnt
		} else {
			details.ComputeCount = &cnt
		}
	}
	if tbs, ok, err := adb.OptionalInt("data_storage_in_tbs", inputs); err != nil {
		return adb.ErrorResult(err.Error()), nil
	} else if ok {
		details.DataStorageSizeInTBs = &tbs
	}
	if strings.TrimSpace(adb.OptionalString("is_auto_scaling_enabled", inputs)) != "" {
		as := adb.OptionalBool("is_auto_scaling_enabled", inputs, false)
		details.IsAutoScalingEnabled = &as
	}
	if strings.TrimSpace(adb.OptionalString("is_auto_scaling_for_storage_enabled", inputs)) != "" {
		ass := adb.OptionalBool("is_auto_scaling_for_storage_enabled", inputs, false)
		details.IsAutoScalingForStorageEnabled = &ass
	}

	resp, err := client.UpdateAutonomousDatabase(adb.Context(), database.UpdateAutonomousDatabaseRequest{
		AutonomousDatabaseId:            &id,
		UpdateAutonomousDatabaseDetails: details,
	})
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	db := adb.SummariseAutonomousDatabase(&resp.AutonomousDatabase)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Scale requested for Autonomous Database %q (now %s)", db["display_name"], db["lifecycle_state"]),
		"database":        db,
		"lifecycle_state": db["lifecycle_state"],
		"work_request_id": adb.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
