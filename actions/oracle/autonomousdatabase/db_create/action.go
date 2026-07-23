// Package oracle_autonomousdatabase_db_create provisions a new Oracle Cloud
// Autonomous Database. The operation is asynchronous — it returns immediately with
// a work-request id while the database transitions PROVISIONING → AVAILABLE.
package oracle_autonomousdatabase_db_create

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
	Name         = "OCI Autonomous Database: Create"
	Description  = "Provision a new Oracle Cloud Autonomous Database in a compartment. Asynchronous — returns a work-request id while the database provisions. Set 'Always Free' for a no-cost database, or choose the workload, compute and storage."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+plus"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "db_name", Type: core.ConnectionTypeString, Label: "Database Name", Placeholder: "Alphanumeric, e.g. SALESDB (no spaces, ≤30 chars)", Required: true},
	{Name: "admin_password", Type: core.ConnectionTypeSecret, Label: "ADMIN Password", Placeholder: "12–30 chars, incl. upper, lower and a digit", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "Friendly name shown in the console (optional)"},
	{Name: "db_workload", Type: core.ConnectionTypeString, Label: "Workload", Placeholder: "Default OLTP", Options: []core.ConnectionOption{
		{Name: "Transaction Processing (OLTP)", Value: "OLTP"},
		{Name: "Data Warehouse (DW)", Value: "DW"},
		{Name: "JSON Database (AJD)", Value: "AJD"},
		{Name: "APEX", Value: "APEX"},
	}},
	{Name: "is_free_tier", Type: core.ConnectionTypeBoolean, Label: "Always Free", Placeholder: "Provision a no-cost Always Free database (fixed small sizing)"},
	{Name: "compute_model", Type: core.ConnectionTypeString, Label: "Compute Model", Placeholder: "Default ECPU", Options: []core.ConnectionOption{
		{Name: "ECPU", Value: "ECPU"},
		{Name: "OCPU", Value: "OCPU"},
	}},
	{Name: "cpu_count", Type: core.ConnectionTypeString, Label: "CPU Count", Placeholder: "ECPU or OCPU count, e.g. 2 (ignored for Always Free)"},
	{Name: "data_storage_in_tbs", Type: core.ConnectionTypeString, Label: "Storage (TB)", Placeholder: "Data storage in TB, e.g. 1 (ignored for Always Free)"},
	{Name: "is_auto_scaling_enabled", Type: core.ConnectionTypeBoolean, Label: "Auto-scaling", Placeholder: "Allow compute auto-scaling up to 3x"},
	{Name: "license_model", Type: core.ConnectionTypeString, Label: "License Model", Placeholder: "Optional", Options: []core.ConnectionOption{
		{Name: "License Included", Value: "LICENSE_INCLUDED"},
		{Name: "Bring Your Own License", Value: "BRING_YOUR_OWN_LICENSE"},
	}},
	{Name: "whitelisted_ips", Type: core.ConnectionTypeString, Label: "Allowed IPs", Placeholder: "Comma-separated IPs/CIDRs to allow (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "database", Type: core.ConnectionTypeObject, Label: "Database"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Database OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := adb.GetAuth(inputs)
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	dbName, err := adb.RequiredString("db_name", inputs)
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	adminPassword, err := adb.RequiredString("admin_password", inputs)
	if err != nil {
		return adb.ErrorResult("ADMIN password is required"), nil
	}

	details := database.CreateAutonomousDatabaseDetails{
		CompartmentId: &compartment,
		DbName:        &dbName,
		AdminPassword: &adminPassword,
	}
	if v := strings.TrimSpace(adb.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	workload := strings.TrimSpace(adb.OptionalString("db_workload", inputs))
	if workload == "" {
		workload = "OLTP"
	}
	details.DbWorkload = database.CreateAutonomousDatabaseBaseDbWorkloadEnum(workload)

	freeTier := adb.OptionalBool("is_free_tier", inputs, false)
	if freeTier {
		free := true
		details.IsFreeTier = &free
	}
	// Compute model, CPU count and storage only apply to paid databases — Always
	// Free is fixed-size, and sending explicit sizing alongside is_free_tier makes
	// OCI reject the request. Skip them entirely when Always Free is set.
	if !freeTier {
		model := strings.TrimSpace(adb.OptionalString("compute_model", inputs))
		if model != "" {
			details.ComputeModel = database.CreateAutonomousDatabaseBaseComputeModelEnum(model)
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
	}
	if adb.OptionalBool("is_auto_scaling_enabled", inputs, false) {
		as := true
		details.IsAutoScalingEnabled = &as
	}
	if v := strings.TrimSpace(adb.OptionalString("license_model", inputs)); v != "" {
		details.LicenseModel = database.CreateAutonomousDatabaseBaseLicenseModelEnum(v)
	}
	if ips := adb.InputStrings("whitelisted_ips", inputs); len(ips) > 0 {
		details.WhitelistedIps = ips
	}
	tags, err := adb.FreeformTags("tags", inputs)
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	details.FreeformTags = tags

	client, err := auth.DatabaseClient()
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.CreateAutonomousDatabase(adb.Context(), database.CreateAutonomousDatabaseRequest{
		CreateAutonomousDatabaseDetails: details,
	})
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	db := adb.SummariseAutonomousDatabase(&resp.AutonomousDatabase)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Provisioning Autonomous Database %q (%s)", db["display_name"], db["lifecycle_state"]),
		"database":        db,
		"id":              db["id"],
		"lifecycle_state": db["lifecycle_state"],
		"work_request_id": adb.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
