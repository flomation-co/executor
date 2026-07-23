// Package oracle_exadata_cloud_vm_cluster_create provisions a cloud VM cluster on existing
// Exadata infrastructure — the set of database VMs that host your databases. Asynchronous:
// it returns the cluster in a PROVISIONING state; poll Get Cloud VM Cluster until AVAILABLE.
package oracle_exadata_cloud_vm_cluster_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	exa "flomation.app/automate/executor/actions/oracle/exadata"

	db "github.com/oracle/oci-go-sdk/v65/database"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Exadata: Create Cloud VM Cluster"
	Description  = "Provision a cloud VM cluster on existing Exadata infrastructure — the database VMs that host your Oracle databases. Give it a subnet, a backup subnet, a hostname, an SSH key, a CPU core count and a Grid Infrastructure version. Asynchronous — poll Get Cloud VM Cluster until AVAILABLE."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (use the tenancy OCID for the root)", Required: true},
	{Name: "display_name", Type: core.ConnectionTypeString, Label: "Display Name", Placeholder: "A friendly name for the VM cluster", Required: true},
	{Name: "cloud_exadata_infrastructure_ocid", Type: core.ConnectionTypeString, Label: "Cloud Exadata Infrastructure OCID", Placeholder: "ocid1.cloudexadatainfrastructure.oc1..aaaa… to run on", Required: true},
	{Name: "subnet_ocid", Type: core.ConnectionTypeString, Label: "Client Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… for client access", Required: true},
	{Name: "backup_subnet_ocid", Type: core.ConnectionTypeString, Label: "Backup Subnet OCID", Placeholder: "ocid1.subnet.oc1..aaaa… for backups", Required: true},
	{Name: "hostname", Type: core.ConnectionTypeString, Label: "Hostname", Placeholder: "The cluster hostname prefix", Required: true},
	{Name: "cpu_core_count", Type: core.ConnectionTypeString, Label: "CPU Core Count", Placeholder: "Total OCPU cores across the cluster", Required: true},
	{Name: "gi_version", Type: core.ConnectionTypeString, Label: "Grid Infrastructure Version", Placeholder: "e.g. 19.0.0.0 — the Oracle Grid Infrastructure version", Required: true},
	{Name: "ssh_public_keys", Type: core.ConnectionTypeText, Label: "SSH Public Keys", Placeholder: "One or more SSH public keys, comma-separated", Required: true},
	{Name: "cluster_name", Type: core.ConnectionTypeString, Label: "Cluster Name", Placeholder: "The Grid Infrastructure cluster name (optional)"},
	{Name: "domain", Type: core.ConnectionTypeString, Label: "Domain", Placeholder: "A domain for the cluster (optional)"},
	{Name: "tags", Type: core.ConnectionTypeString, Label: "Freeform Tags (JSON)", Placeholder: `{"env":"prod"} (optional)`},
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
	compartment, err := auth.RequiredCompartment()
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	req := map[string]func() (string, error){
		"display_name":                      func() (string, error) { return exa.RequiredString("display_name", inputs) },
		"cloud_exadata_infrastructure_ocid": func() (string, error) { return exa.RequiredString("cloud_exadata_infrastructure_ocid", inputs) },
		"subnet_ocid":                       func() (string, error) { return exa.RequiredString("subnet_ocid", inputs) },
		"backup_subnet_ocid":                func() (string, error) { return exa.RequiredString("backup_subnet_ocid", inputs) },
		"hostname":                          func() (string, error) { return exa.RequiredString("hostname", inputs) },
		"gi_version":                        func() (string, error) { return exa.RequiredString("gi_version", inputs) },
	}
	vals := map[string]string{}
	for k, fn := range req {
		v, err := fn()
		if err != nil {
			return exa.ErrorResult(err.Error()), nil
		}
		vals[k] = v
	}
	cpuCores, err := exa.RequiredInt("cpu_core_count", inputs)
	if err != nil {
		return exa.ErrorResult(err.Error()), nil
	}
	keys := exa.InputStrings("ssh_public_keys", inputs)
	if len(keys) == 0 {
		return exa.ErrorResult("at least one SSH public key is required"), nil
	}
	displayName := vals["display_name"]
	infra := vals["cloud_exadata_infrastructure_ocid"]
	subnet := vals["subnet_ocid"]
	backupSubnet := vals["backup_subnet_ocid"]
	hostname := vals["hostname"]
	giVersion := vals["gi_version"]
	details := db.CreateCloudVmClusterDetails{
		CompartmentId:                &compartment,
		DisplayName:                  &displayName,
		CloudExadataInfrastructureId: &infra,
		SubnetId:                     &subnet,
		BackupSubnetId:               &backupSubnet,
		Hostname:                     &hostname,
		GiVersion:                    &giVersion,
		CpuCoreCount:                 &cpuCores,
		SshPublicKeys:                keys,
	}
	if v := exa.OptionalString("cluster_name", inputs); v != "" {
		details.ClusterName = &v
	}
	if v := exa.OptionalString("domain", inputs); v != "" {
		details.Domain = &v
	}
	if tags, err := exa.FreeformTags("tags", inputs); err != nil {
		return exa.ErrorResult(err.Error()), nil
	} else {
		details.FreeformTags = tags
	}
	resp, err := client.CreateCloudVmCluster(exa.Context(), db.CreateCloudVmClusterRequest{CreateCloudVmClusterDetails: details})
	if err != nil {
		return exa.ErrorResult(auth.OCIError(err)), nil
	}
	cluster := exa.SummariseCloudVmCluster(&resp.CloudVmCluster)
	return exa.Result(fmt.Sprintf("Provisioning VM cluster %q (%s) — poll Get Cloud VM Cluster until AVAILABLE", displayName, cluster["lifecycle_state"]), map[string]interface{}{
		"vm_cluster": cluster, "id": cluster["id"], "lifecycle_state": cluster["lifecycle_state"], "work_request_id": exa.Str(resp.OpcWorkRequestId),
	}), nil
}
