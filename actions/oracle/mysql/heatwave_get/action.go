// Package oracle_mysql_heatwave_get fetches the HeatWave cluster attached to a MySQL DB System by
// the DB System's OCID, returning its shape, node count and lifecycle state.
package oracle_mysql_heatwave_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"

	"github.com/oracle/oci-go-sdk/v65/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: Get HeatWave Cluster"
	Description  = "Fetch the HeatWave cluster attached to a MySQL DB System — its shape, node count and lifecycle state."
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
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "heatwave_cluster", Type: core.ConnectionTypeObject, Label: "HeatWave Cluster"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
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

	resp, err := client.GetHeatWaveCluster(my.Context(), mysql.GetHeatWaveClusterRequest{DbSystemId: &dbSystemID})
	if err != nil {
		return my.ErrorResult(auth.OCIError(err)), nil
	}
	cluster := my.SummariseHeatWaveCluster(&resp.HeatWaveCluster)
	return my.Result(
		fmt.Sprintf("HeatWave cluster on DB System %q — %v node(s), %s", cluster["shape_name"], cluster["node_count"], cluster["lifecycle_state"]),
		map[string]interface{}{
			"heatwave_cluster": cluster,
			"lifecycle_state":  cluster["lifecycle_state"],
		}), nil
}
