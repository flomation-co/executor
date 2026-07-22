// Package oracle_mysql_backup_create takes an on-demand backup of a MySQL HeatWave DB system.
// Asynchronous: the backup comes back CREATING with a work-request id; poll Get Backup until it
// is ACTIVE before relying on it.
package oracle_mysql_backup_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"

	"github.com/oracle/oci-go-sdk/v65/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: Create Backup"
	Description  = "Take an on-demand backup of a MySQL HeatWave DB system. Returns the backup in a CREATING state plus a work-request id — poll Get Backup until ACTIVE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+database"
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
	{Name: "db_system_ocid", Type: core.ConnectionTypeString, Label: "DB System OCID", Placeholder: "ocid1.mysqldbsystem.oc1..aaaa…", Required: true},
	{Name: "backup_type", Type: core.ConnectionTypeString, Label: "Backup Type", Placeholder: "FULL (default) or INCREMENTAL", Options: []core.ConnectionOption{
		{Name: "Full", Value: "FULL"},
		{Name: "Incremental", Value: "INCREMENTAL"},
	}},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A name for the backup (optional)"},
	{Name: "retention_days", Type: core.ConnectionTypeString, Label: "Retention (days)", Placeholder: "Number of days to retain this backup (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backup", Type: core.ConnectionTypeObject, Label: "Backup"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Backup OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := my.BackupsClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	dbSystemID, err := my.RequiredString("db_system_ocid", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}

	details := mysql.CreateBackupDetails{DbSystemId: &dbSystemID}
	if bt := my.OptionalString("backup_type", inputs); bt != "" {
		details.BackupType = mysql.CreateBackupDetailsBackupTypeEnum(bt)
	}
	if name := my.OptionalString("display_name", inputs); name != "" {
		details.DisplayName = &name
	}
	if n, ok, err := my.OptionalInt("retention_days", inputs); err != nil {
		return my.ErrorResult(err.Error()), nil
	} else if ok {
		details.RetentionInDays = &n
	}

	resp, err := client.CreateBackup(my.Context(), mysql.CreateBackupRequest{CreateBackupDetails: details})
	if err != nil {
		return my.ErrorResult(auth.OCIError(err)), nil
	}
	return my.Result(fmt.Sprintf("Creating backup of DB system %q — poll Get Backup until ACTIVE", dbSystemID), map[string]interface{}{
		"backup":          my.SummariseBackup(&resp.Backup),
		"id":              my.Str(resp.Backup.Id),
		"lifecycle_state": string(resp.Backup.LifecycleState),
		"work_request_id": my.Str(resp.OpcWorkRequestId),
	}), nil
}
