// Package oracle_mysql_heatwave_add attaches a HeatWave cluster to a MySQL DB system for in-memory
// analytics acceleration. Asynchronous: the add returns a work-request id and the cluster comes up in
// the background — poll the work request (or Get HeatWave Cluster) until it is ACTIVE.
package oracle_mysql_heatwave_add

import (
	"fmt"

	core "flomation.app/automate/executor"
	my "flomation.app/automate/executor/actions/oracle/mysql"

	"github.com/oracle/oci-go-sdk/v65/mysql"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI MySQL: Add HeatWave Cluster"
	Description  = "Attach a HeatWave cluster to a MySQL DB system for in-memory analytics. Returns a work-request id — poll until the cluster is ACTIVE."
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
	{Name: "cluster_size", Type: core.ConnectionTypeString, Label: "Cluster Size (nodes)", Placeholder: "Number of HeatWave nodes to provision, e.g. 1", Required: true},
	{Name: "shape_name", Type: core.ConnectionTypeString, Label: "Shape Name", Placeholder: "HeatWave node shape, e.g. MySQL.HeatWave.VM.Standard.E3", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
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
	clusterSize, err := my.RequiredInt("cluster_size", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}
	shapeName, err := my.RequiredString("shape_name", inputs)
	if err != nil {
		return my.ErrorResult(err.Error()), nil
	}

	resp, err := client.AddHeatWaveCluster(my.Context(), mysql.AddHeatWaveClusterRequest{
		DbSystemId: &dbSystemID,
		AddHeatWaveClusterDetails: mysql.AddHeatWaveClusterDetails{
			ShapeName:   &shapeName,
			ClusterSize: &clusterSize,
		},
	})
	if err != nil {
		return my.ErrorResult(auth.OCIError(err)), nil
	}
	return my.Result(fmt.Sprintf("Adding a %d-node HeatWave cluster (%s) to DB system %s — poll the work request until ACTIVE", clusterSize, shapeName, dbSystemID), map[string]interface{}{
		"work_request_id": my.Str(resp.OpcWorkRequestId),
	}), nil
}
