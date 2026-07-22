// Package oracle_exadata_cloud_vm_cluster_remove_vm removes one or more DB servers'
// virtual machines from a cloud VM cluster — the cluster enters UPDATING and a
// work-request tracks the scale-down; poll Get until the lifecycle state settles.
package oracle_exadata_cloud_vm_cluster_remove_vm

import (
	"fmt"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Remove VM from Cloud VM Cluster"
	Description  = "Remove one or more DB servers' virtual machines from a cloud VM cluster — asynchronous, returns the cluster in UPDATING plus a work-request id."
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
	{Name: "db_server_ocids", Type: core.ConnectionTypeString, Label: "DB Server OCIDs", Placeholder: "Comma-separated ocid1.dbserver.oc1..aaaa… to remove", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "vm_cluster", Type: core.ConnectionTypeObject, Label: "VM Cluster"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "VM Cluster OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
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
	dbServers := exa.InputStrings("db_server_ocids", inputs)
	if len(dbServers) == 0 {
		return exa.ErrorResult("db server ocids is required"), nil
	}
	servers := make([]db.CloudDbServerDetails, 0, len(dbServers))
	for _, s := range dbServers {
		s := s
		servers = append(servers, db.CloudDbServerDetails{DbServerId: &s})
	}
	details := db.RemoveVirtualMachineFromCloudVmClusterDetails{DbServers: servers}
	resp, err := client.RemoveVirtualMachineFromCloudVmCluster(exa.Context(), db.RemoveVirtualMachineFromCloudVmClusterRequest{
		CloudVmClusterId: &id,
		RemoveVirtualMachineFromCloudVmClusterDetails: details,
	})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	vc := exa.SummariseCloudVmCluster(&resp.CloudVmCluster)
	return exa.Result(fmt.Sprintf("Removing %d VM(s) from cloud VM cluster %q — now %s", len(servers), vc["display_name"], vc["lifecycle_state"]), map[string]interface{}{
		"vm_cluster":      vc,
		"id":              vc["id"],
		"lifecycle_state": vc["lifecycle_state"],
		"work_request_id": exa.Str(resp.OpcWorkRequestId),
	}), nil
}
