// Package oracle_autonomousdatabase_db_restore restores an Oracle Cloud
// Autonomous Database to a point in time — either an explicit timestamp or the
// latest available restore point. The operation is asynchronous — it returns
// immediately with a work-request id while the database is restored.
package oracle_autonomousdatabase_db_restore

import (
	"fmt"
	"time"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: Restore"
	Description  = "Restore an Oracle Cloud Autonomous Database to a point in time (a timestamp, or the latest restore point). Asynchronous — returns a work-request id."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+clock-rotate-left"
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
	{Name: "autonomous_database_id", Type: core.ConnectionTypeString, Label: "Autonomous Database OCID", Placeholder: "ocid1.autonomousdatabase.oc1..aaaa…", Required: true},
	{Name: "restore_timestamp", Type: core.ConnectionTypeString, Label: "Restore Timestamp", Placeholder: "Point in time to restore to, RFC3339 e.g. 2026-07-21T14:30:00Z (optional)"},
	{Name: "restore_latest", Type: core.ConnectionTypeBoolean, Label: "Restore to Latest", Placeholder: "Restore to the last known good state with the least possible data loss"},
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

	details := database.RestoreAutonomousDatabaseDetails{}
	if v := adb.OptionalString("restore_timestamp", inputs); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return adb.ErrorResult(fmt.Sprintf("invalid restore timestamp %q: expected RFC3339 (e.g. 2026-07-21T14:30:00Z)", v)), nil
		}
		details.Timestamp = &common.SDKTime{Time: t}
	} else if adb.OptionalBool("restore_latest", inputs, false) {
		latest := true
		details.Latest = &latest
	} else {
		return adb.ErrorResult("provide a restore timestamp or set restore latest"), nil
	}

	resp, err := client.RestoreAutonomousDatabase(adb.Context(), database.RestoreAutonomousDatabaseRequest{
		AutonomousDatabaseId:             &id,
		RestoreAutonomousDatabaseDetails: details,
	})
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}

	db := adb.SummariseAutonomousDatabase(&resp.AutonomousDatabase)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Restore requested for Autonomous Database %q (now %s)", db["display_name"], db["lifecycle_state"]),
		"database":        db,
		"lifecycle_state": db["lifecycle_state"],
		"work_request_id": adb.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
