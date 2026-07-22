// Package oracle_exadata_cloud_vm_cluster_iorm_config_get reads the I/O Resource
// Management (IORM) configuration for one cloud VM cluster — its objective, lifecycle
// state and the per-database share plans.
package oracle_exadata_cloud_vm_cluster_iorm_config_get

import (
	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Get VM Cluster IORM Config"
	Description  = "Fetch the I/O Resource Management (IORM) configuration for a cloud VM cluster — its objective, lifecycle state and per-database share plans."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microchip"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "cloud_vm_cluster_ocid", Type: core.ConnectionTypeString, Label: "Cloud VM Cluster OCID", Placeholder: "ocid1.cloudvmcluster.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "objective", Type: core.ConnectionTypeString, Label: "Objective"},
	{Name: "db_plans", Type: core.ConnectionTypeObject, Label: "Database Plans"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := exa.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := exa.RequiredString("cloud_vm_cluster_ocid", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetCloudVmClusterIormConfig(exa.Context(), db.GetCloudVmClusterIormConfigRequest{CloudVmClusterId: &id})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	plans := make([]map[string]interface{}, 0, len(resp.DbPlans))
	for _, p := range resp.DbPlans {
		plans = append(plans, map[string]interface{}{
			"db_name": exa.Str(p.DbName),
			"share":   exa.IntOrNil(p.Share),
		})
	}
	return exa.Result("IORM config", map[string]interface{}{
		"lifecycle_state": string(resp.LifecycleState),
		"objective":       string(resp.Objective),
		"db_plans":        plans,
	}), nil
}
