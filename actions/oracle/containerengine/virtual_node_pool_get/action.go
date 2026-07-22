// Package oracle_containerengine_virtual_node_pool_get reads one OKE virtual node pool by
// OCID, including its cluster, Kubernetes version, size, and lifecycle state.
package oracle_containerengine_virtual_node_pool_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Get Virtual Node Pool"
	Description  = "Fetch a single Oracle Cloud OKE virtual node pool by OCID — its cluster, Kubernetes version, size, and lifecycle state."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "virtual_node_pool_ocid", Type: core.ConnectionTypeString, Label: "Virtual Node Pool OCID", Placeholder: "ocid1.virtualnodepool.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "virtual_node_pool", Type: core.ConnectionTypeObject, Label: "Virtual Node Pool"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Virtual Node Pool OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := oke.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	id, err := oke.RequiredString("virtual_node_pool_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetVirtualNodePool(oke.Context(), okesdk.GetVirtualNodePoolRequest{VirtualNodePoolId: &id})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	pool := oke.SummariseVirtualNodePool(&resp.VirtualNodePool)
	return oke.Result(fmt.Sprintf("Virtual node pool %q is %s", pool["display_name"], pool["lifecycle_state"]), map[string]interface{}{
		"virtual_node_pool": pool, "id": pool["id"], "lifecycle_state": pool["lifecycle_state"],
	}), nil
}
