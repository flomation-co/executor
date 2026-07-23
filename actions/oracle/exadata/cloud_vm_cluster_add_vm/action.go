// Package oracle_exadata_cloud_vm_cluster_add_vm scales a cloud VM cluster out by adding
// virtual machines to it, one per supplied DB server. This is asynchronous — it returns the
// cluster in an UPDATING state plus a work-request id; poll Get Cloud VM Cluster until the
// cluster is AVAILABLE again.
package oracle_exadata_cloud_vm_cluster_add_vm

import (
	"fmt"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Add VM to Cloud VM Cluster"
	Description  = "Scale a cloud VM cluster out by adding virtual machines — one per supplied ExaDB-D DB server OCID. Asynchronous — poll Get Cloud VM Cluster until AVAILABLE."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+microchip"
	Date         = "22/07/2026"
	Type         = core.ActionTypeAction
)

var Inputs = [...]core.Connection{
	// Managed "Connect Oracle Cloud" credential (default); the raw API signing key is the advanced fallback. Picking a credential auto-fills the hidden signing fields, so the executor reads the same inputs either way.
	{Name: "auth_method", Type: core.ConnectionTypeString, Label: "Authentication", Options: []core.ConnectionOption{{Name: "Connect Oracle Cloud", Value: "connect"}, {Name: "API signing key (advanced)", Value: "keys"}}},
	{Name: "credential", Type: core.ConnectionTypeCredential, Label: "Oracle Cloud connection", Placeholder: "Pick a connected Oracle Cloud account", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"", "connect"}}},
	{Name: "tenancy_ocid", Type: core.ConnectionTypeString, Label: "Tenancy OCID", Placeholder: "ocid1.tenancy.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "user_ocid", Type: core.ConnectionTypeString, Label: "User OCID", Placeholder: "ocid1.user.oc1..aaaa…", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "region", Type: core.ConnectionTypeString, Label: "Region", Placeholder: "e.g. uk-london-1", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "fingerprint", Type: core.ConnectionTypeString, Label: "Key Fingerprint", Placeholder: "aa:bb:cc:… fingerprint of the uploaded API key", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key", Type: core.ConnectionTypeSecret, Label: "Private Key (PEM)", Placeholder: "The API signing private key — full PEM, incl. BEGIN/END lines", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "private_key_passphrase", Type: core.ConnectionTypeSecret, Label: "Private Key Passphrase", Placeholder: "Only if the key is encrypted (optional)", Visible: &core.VisibleWhen{Field: "auth_method", Values: []string{"keys"}}},
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "cloud_vm_cluster_ocid", Type: core.ConnectionTypeString, Label: "Cloud VM Cluster OCID", Placeholder: "ocid1.cloudvmcluster.oc1..aaaa…", Required: true},
	{Name: "db_server_ocids", Type: core.ConnectionTypeString, Label: "DB Server OCIDs", Placeholder: "Comma-separated ExaDB-D DB server OCIDs to add as VMs", Required: true},
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
	details := db.AddVirtualMachineToCloudVmClusterDetails{}
	for _, s := range dbServers {
		s := s
		details.DbServers = append(details.DbServers, db.CloudDbServerDetails{DbServerId: &s})
	}
	resp, err := client.AddVirtualMachineToCloudVmCluster(exa.Context(), db.AddVirtualMachineToCloudVmClusterRequest{
		CloudVmClusterId:                         &id,
		AddVirtualMachineToCloudVmClusterDetails: details,
	})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	cluster := exa.SummariseCloudVmCluster(&resp.CloudVmCluster)
	return exa.Result(fmt.Sprintf("Adding %d VM(s) to cloud VM cluster %q (%s) — poll Get Cloud VM Cluster until AVAILABLE", len(dbServers), cluster["display_name"], cluster["lifecycle_state"]), map[string]interface{}{
		"vm_cluster": cluster, "id": cluster["id"], "lifecycle_state": cluster["lifecycle_state"], "work_request_id": exa.Str(resp.OpcWorkRequestId),
	}), nil
}
