// Package oracle_autonomousdatabase_db_create_backup creates a backup of an Oracle
// Cloud Autonomous Database. It defaults to a long-term backup (manual backups are
// deprecated on Autonomous Database Serverless), with a 90-day retention unless one
// is given. Asynchronous — it returns immediately with a work-request id.
package oracle_autonomousdatabase_db_create_backup

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
	Name         = "OCI Autonomous Database: Create Backup"
	Description  = "Create a backup of an Oracle Cloud Autonomous Database. Defaults to a long-term backup (manual backups are deprecated on Autonomous Database Serverless) retained for 90 days unless you set a retention period. Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+floppy-disk"
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Backup Name", Placeholder: "Friendly name for the backup (optional)"},
	{Name: "retention_period_in_days", Type: core.ConnectionTypeString, Label: "Retention Period (days)", Placeholder: "Long-term retention in days, 90–3650 (default 90)"},
	{Name: "is_long_term_backup", Type: core.ConnectionTypeBoolean, Label: "Long-term Backup", Placeholder: "On by default — required on Serverless; turn off only for dedicated infrastructure"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backup", Type: core.ConnectionTypeObject, Label: "Backup"},
	{Name: "backup_id", Type: core.ConnectionTypeString, Label: "Backup OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request ID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := adb.PerDatabaseClient(inputs)
	if errResult != nil {
		return errResult, nil
	}

	details := database.CreateAutonomousDatabaseBackupDetails{
		AutonomousDatabaseId: &id,
	}
	if v := strings.TrimSpace(adb.OptionalString("display_name", inputs)); v != "" {
		details.DisplayName = &v
	}
	// Manual backups are deprecated on Autonomous Database Serverless (the common
	// case), so default to a long-term backup unless the operator explicitly turns
	// it off. Long-term backups need a retention period; default to the 90-day
	// minimum when one isn't given.
	longTerm := adb.OptionalBool("is_long_term_backup", inputs, true)
	details.IsLongTermBackup = &longTerm
	retention := 90
	if days, ok, err := adb.OptionalInt("retention_period_in_days", inputs); err != nil {
		return adb.ErrorResult(err.Error()), nil
	} else if ok {
		retention = days
	}
	if longTerm {
		details.RetentionPeriodInDays = &retention
	}

	resp, err := client.CreateAutonomousDatabaseBackup(adb.Context(), database.CreateAutonomousDatabaseBackupRequest{
		CreateAutonomousDatabaseBackupDetails: details,
	})
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}

	summary := adb.SummariseBackup(&resp.AutonomousDatabaseBackup)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Backup requested for Autonomous Database — backup %q (%s)", summary["display_name"], summary["lifecycle_state"]),
		"backup":          summary,
		"backup_id":       summary["id"],
		"work_request_id": adb.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
