// Package oracle_mysql_heatwave_delete removes the HeatWave cluster attached to a MySQL DB system,
// leaving the DB system itself in place. Asynchronous: the DB system goes to UPDATING while the
// cluster is torn down, returning a work-request id you can poll for completion.
package oracle_mysql_heatwave_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"

	"github.com/oracle/oci-go-sdk/v65/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: Delete HeatWave Cluster"
	Description  = "Delete the HeatWave cluster attached to a MySQL DB system. Returns a work-request id — poll it until the removal completes."
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
	{Name: "db_system_ocid", Type: core.ConnectionTypeString, Label: "DB System OCID", Placeholder: "ocid1.mysqldbsystem.oc1..aaaa… of the DB system whose HeatWave cluster to delete", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "DB System OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := my.HeatWaveClient(inputs)
	if errResult != nil {
		return errResult, nil
	}
	dbSystemID, err := my.RequiredString("db_system_ocid", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}

	resp, err := client.DeleteHeatWaveCluster(my.Context(), mysql.DeleteHeatWaveClusterRequest{DbSystemId: &dbSystemID})
	if err != nil {
		return my.ErrorResult(auth.OCIError(err)), nil
	}
	return my.Result(fmt.Sprintf("Deleting HeatWave cluster on DB system %s — poll the work request until it completes", dbSystemID), map[string]interface{}{
		"id":              dbSystemID,
		"work_request_id": my.Str(resp.OpcWorkRequestId),
	}), nil
}
