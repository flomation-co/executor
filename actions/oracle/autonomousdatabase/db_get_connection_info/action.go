// Package oracle_autonomousdatabase_db_get_connection_info surfaces the
// connection strings and service console URLs for one Oracle Cloud Autonomous
// Database — everything a client or tool needs to connect, without a wallet.
package oracle_autonomousdatabase_db_get_connection_info

import (
	"fmt"

	core "flomation.app/automate/executor"
	adb "flomation.app/automate/executor/actions/oracle/autonomousdatabase"

	"github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Autonomous Database: Get Connection Info"
	Description  = "Get the connection strings and service console URLs for an Oracle Cloud Autonomous Database — everything a client or tool needs to connect, without downloading a wallet."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+plug"
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
	{Name: "connection_strings", Type: core.ConnectionTypeObject, Label: "Connection Strings"},
	{Name: "connection_urls", Type: core.ConnectionTypeObject, Label: "Connection URLs"},
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

	connStrings := map[string]interface{}{}
	if cs := resp.AutonomousDatabase.ConnectionStrings; cs != nil {
		if cs.High != nil {
			connStrings["high"] = *cs.High
		}
		if cs.Medium != nil {
			connStrings["medium"] = *cs.Medium
		}
		if cs.Low != nil {
			connStrings["low"] = *cs.Low
		}
		if cs.Dedicated != nil {
			connStrings["dedicated"] = *cs.Dedicated
		}
		if len(cs.AllConnectionStrings) > 0 {
			all := map[string]interface{}{}
			for k, v := range cs.AllConnectionStrings {
				all[k] = v
			}
			connStrings["all_connection_strings"] = all
		}
	}

	connUrls := map[string]interface{}{}
	if cu := resp.AutonomousDatabase.ConnectionUrls; cu != nil {
		if cu.SqlDevWebUrl != nil {
			connUrls["sql_dev_web_url"] = *cu.SqlDevWebUrl
		}
		if cu.ApexUrl != nil {
			connUrls["apex_url"] = *cu.ApexUrl
		}
		if cu.MachineLearningUserManagementUrl != nil {
			connUrls["machine_learning_user_management_url"] = *cu.MachineLearningUserManagementUrl
		}
		if cu.GraphStudioUrl != nil {
			connUrls["graph_studio_url"] = *cu.GraphStudioUrl
		}
		if cu.MongoDbUrl != nil {
			connUrls["mongo_db_url"] = *cu.MongoDbUrl
		}
		if cu.MachineLearningNotebookUrl != nil {
			connUrls["machine_learning_notebook_url"] = *cu.MachineLearningNotebookUrl
		}
		if cu.OrdsUrl != nil {
			connUrls["ords_url"] = *cu.OrdsUrl
		}
		if cu.DatabaseTransformsUrl != nil {
			connUrls["database_transforms_url"] = *cu.DatabaseTransformsUrl
		}
		if cu.SpatialStudioUrl != nil {
			connUrls["spatial_studio_url"] = *cu.SpatialStudioUrl
		}
	}

	lifecycleState := string(resp.AutonomousDatabase.LifecycleState)
	return map[string]interface{}{
		"tool_result":        fmt.Sprintf("Retrieved %d connection string field(s) and %d connection URL(s) for the Autonomous Database", len(connStrings), len(connUrls)),
		"connection_strings": connStrings,
		"connection_urls":    connUrls,
		"lifecycle_state":    lifecycleState,
		"success":            true,
	}, nil
}
