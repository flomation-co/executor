// Package oracle_autonomousdatabase_db_get_backup fetches the details of one
// Oracle Cloud Autonomous Database backup by OCID — its type, lifecycle state,
// size, timing and restorability.
package oracle_autonomousdatabase_db_get_backup

import (
	"fmt"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: Get Backup"
	Description  = "Fetch the details of one Oracle Cloud Autonomous Database backup by OCID."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+circle-info"
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
	{Name: "autonomous_database_backup_id", Type: core.ConnectionTypeString, Label: "Autonomous Database Backup OCID", Placeholder: "ocid1.autonomousdatabasebackup.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backup", Type: core.ConnectionTypeObject, Label: "Backup"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, err := adb.GetAuth(inputs)
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	id, err := adb.RequiredString("autonomous_database_backup_id", inputs)
	if err != nil {
		return adb.ErrorResult(err.Error()), nil
	}
	client, err := auth.DatabaseClient()
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	resp, err := client.GetAutonomousDatabaseBackup(adb.Context(), database.GetAutonomousDatabaseBackupRequest{AutonomousDatabaseBackupId: &id})
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}

	backup := adb.SummariseBackup(&resp.AutonomousDatabaseBackup)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Autonomous Database backup %q is %s", backup["display_name"], backup["lifecycle_state"]),
		"backup":          backup,
		"lifecycle_state": backup["lifecycle_state"],
		"success":         true,
	}, nil
}
