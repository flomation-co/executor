// Package oracle_containerengine_node_pool_update changes a managed OKE node pool — rename
// it, upgrade its Kubernetes version, or scale the node count. Asynchronous — it returns a
// work-request id; poll Get Work Request until it completes, then Get Node Pool.
package oracle_containerengine_node_pool_update

import (
	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Update Node Pool"
	Description  = "Change a managed Oracle Cloud OKE node pool — rename it, upgrade its Kubernetes version, or scale the node count. Asynchronous — returns a work-request id; poll Get Work Request until it completes, then Get Node Pool."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+cubes"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the node-pool picker)"},
	{Name: "node_pool_ocid", Type: core.ConnectionTypeString, Label: "Node Pool OCID", Placeholder: "ocid1.nodepool.oc1..aaaa…", Required: true},
	{Name: "name", Type: core.ConnectionTypeString, Label: "Node Pool Name", Placeholder: "A new name for the node pool (optional)"},
	{Name: "kubernetes_version", Type: core.ConnectionTypeString, Label: "Kubernetes Version", Placeholder: "e.g. v1.30.1 to upgrade the nodes to (optional)"},
	{Name: "size", Type: core.ConnectionTypeString, Label: "Node Count", Placeholder: "New number of worker nodes — scales the pool (optional)"},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := oke.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := oke.RequiredString("node_pool_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	details := okesdk.UpdateNodePoolDetails{}
	if name := oke.OptionalString("name", inputs); name != "" {
		details.Name = &name
	}
	if k8sVersion := oke.OptionalString("kubernetes_version", inputs); k8sVersion != "" {
		details.KubernetesVersion = &k8sVersion
	}
	if size, ok, err := oke.OptionalInt("size", inputs); err != nil {
		return oke.ErrorResult(err.Error()), nil
	} else if ok {
		details.NodeConfigDetails = &okesdk.UpdateNodePoolNodeConfigDetails{Size: &size}
	}
	resp, err := client.UpdateNodePool(oke.Context(), okesdk.UpdateNodePoolRequest{NodePoolId: &id, UpdateNodePoolDetails: details})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	return oke.AsyncResult("Updating node pool — poll Get Work Request until it completes", oke.Str(resp.OpcWorkRequestId)), nil
}
