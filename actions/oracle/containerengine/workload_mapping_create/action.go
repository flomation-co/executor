// Package oracle_containerengine_workload_mapping_create creates a workload mapping on an
// OKE cluster, tying a Kubernetes namespace to a mapped customer compartment.
package oracle_containerengine_workload_mapping_create

import (
	"fmt"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Create Workload Mapping"
	Description  = "Create a workload mapping on an Oracle Cloud OKE cluster — bind a Kubernetes namespace to a mapped customer compartment."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+cubes"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the cluster picker)"},
	{Name: "cluster_ocid", Type: core.ConnectionTypeString, Label: "Cluster OCID", Placeholder: "ocid1.cluster.oc1..aaaa…", Required: true},
	{Name: "namespace", Type: core.ConnectionTypeString, Label: "Namespace", Placeholder: "The Kubernetes namespace to map, e.g. default", Required: true},
	{Name: "mapped_compartment_ocid", Type: core.ConnectionTypeString, Label: "Mapped Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… — the customer compartment to map to", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "workload_mapping", Type: core.ConnectionTypeObject, Label: "Workload Mapping"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Workload Mapping OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := oke.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	clusterID, err := oke.RequiredString("cluster_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	ns, err := oke.RequiredString("namespace", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	mappedComp, err := oke.RequiredString("mapped_compartment_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	details := okesdk.CreateWorkloadMappingDetails{Namespace: &ns, MappedCompartmentId: &mappedComp}
	resp, err := client.CreateWorkloadMapping(oke.Context(), okesdk.CreateWorkloadMappingRequest{
		ClusterId:                    &clusterID,
		CreateWorkloadMappingDetails: details,
	})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	mapping := oke.SummariseWorkloadMapping(&resp.WorkloadMapping)
	return oke.Result(fmt.Sprintf("Created workload mapping for namespace %q (%s)", mapping["namespace"], mapping["lifecycle_state"]), map[string]interface{}{
		"workload_mapping": mapping, "id": mapping["id"], "lifecycle_state": mapping["lifecycle_state"],
	}), nil
}
