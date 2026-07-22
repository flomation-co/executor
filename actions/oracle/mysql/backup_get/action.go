// Package oracle_mysql_backup_get fetches a single MySQL HeatWave backup by its OCID, returning its
// type, source DB system, size, MySQL version and lifecycle state.
package oracle_mysql_backup_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"

	"github.com/oracle/oci-go-sdk/v65/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: Get Backup"
	Description  = "Fetch a single MySQL HeatWave backup by its OCID — its type, source DB system, size and lifecycle state."
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
	{Name: "backup_ocid", Type: core.ConnectionTypeString, Label: "Backup OCID", Placeholder: "ocid1.mysqlbackup.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "backup", Type: core.ConnectionTypeObject, Label: "Backup"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Backup OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := my.BackupsClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	backupID, err := my.RequiredString("backup_ocid", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}

	resp, err := client.GetBackup(my.Context(), mysql.GetBackupRequest{BackupId: &backupID})
	if err != nil {
		return my.ErrorResult(auth.OCIError(err)), nil
	}
	backup := my.SummariseBackup(&resp.Backup)
	return my.Result(fmt.Sprintf("Backup %q (%s)", backup["display_name"], backup["lifecycle_state"]), map[string]interface{}{
		"backup": backup, "id": backup["id"], "lifecycle_state": backup["lifecycle_state"],
	}), nil
}
