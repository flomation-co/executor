// Package oracle_autonomousdatabase_db_get fetches the full state of one Oracle
// Cloud Autonomous Database by OCID — lifecycle state, compute/storage, workload
// and metadata.
package oracle_autonomousdatabase_db_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: Get"
	Description  = "Fetch the full state of one Oracle Cloud Autonomous Database by OCID — lifecycle state, compute and storage sizing, workload type and metadata."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+magnifying-glass"
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "database", Type: core.ConnectionTypeObject, Label: "Database"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := adb.PerDatabaseClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.GetAutonomousDatabase(adb.Context(), database.GetAutonomousDatabaseRequest{AutonomousDatabaseId: &id})
	if err != nil {
		return adb.ErrorResult(auth.OCIError(err)), nil
	}
	db := adb.SummariseAutonomousDatabase(&resp.AutonomousDatabase)
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Autonomous Database %q is %s", db["display_name"], db["lifecycle_state"]),
		"database":        db,
		"lifecycle_state": db["lifecycle_state"],
		"success":         true,
	}, nil
}
