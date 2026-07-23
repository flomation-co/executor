// Package oracle_exadata_cloud_vm_cluster_update changes a cloud VM cluster's display name
// and/or scales its CPU cores. Asynchronous: it returns the cluster in an UPDATING state plus
// a work-request id; poll Get Cloud VM Cluster until it settles back to AVAILABLE.
package oracle_exadata_cloud_vm_cluster_update

import (
	"fmt"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Update Cloud VM Cluster"
	Description  = "Rename a cloud VM cluster or scale its total CPU core count. Asynchronous — poll Get Cloud VM Cluster until it settles back to AVAILABLE."
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
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A new friendly name for the VM cluster (optional)"},
	{Name: "cpu_core_count", Type: core.ConnectionTypeString, Label: "CPU Core Count", Placeholder: "New total OCPU cores across the cluster (optional)"},
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
	details := db.UpdateCloudVmClusterDetails{}
	if v := exa.OptionalString("display_name", inputs); v != "" {
		details.DisplayName = &v
	}
	if n, ok, err := exa.OptionalInt("cpu_core_count", inputs); err != nil {
		return exa.ErrorResult(err.Error()), nil
	} else if ok {
		details.CpuCoreCount = &n
	}
	resp, err := client.UpdateCloudVmCluster(exa.Context(), db.UpdateCloudVmClusterRequest{CloudVmClusterId: &id, UpdateCloudVmClusterDetails: details})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	cluster := exa.SummariseCloudVmCluster(&resp.CloudVmCluster)
	return exa.Result(fmt.Sprintf("Updating VM cluster %q (%s) — poll Get Cloud VM Cluster until AVAILABLE", cluster["display_name"], cluster["lifecycle_state"]), map[string]interface{}{
		"vm_cluster": cluster, "id": cluster["id"], "lifecycle_state": cluster["lifecycle_state"], "work_request_id": exa.Str(resp.OpcWorkRequestId),
	}), nil
}
