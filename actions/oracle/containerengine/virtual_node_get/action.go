// Package oracle_containerengine_virtual_node_get reads one OKE virtual node by OCID within
// its virtual node pool, including its availability domain, private IP, and lifecycle state.
package oracle_containerengine_virtual_node_get

import (
	"fmt"

	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Get Virtual Node"
	Description  = "Fetch a single Oracle Cloud OKE virtual node by OCID — its availability domain, subnet, private IP, and lifecycle state."
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the picker)"},
	{Name: "virtual_node_pool_ocid", Type: core.ConnectionTypeString, Label: "Virtual Node Pool OCID", Placeholder: "ocid1.virtualnodepool.oc1..aaaa…", Required: true},
	{Name: "virtual_node_ocid", Type: core.ConnectionTypeString, Label: "Virtual Node OCID", Placeholder: "ocid1.virtualnode.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "virtual_node", Type: core.ConnectionTypeObject, Label: "Virtual Node"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Virtual Node OCID"},
	{Name: "lifecycle_state", Type: core.ConnectionTypeString, Label: "Lifecycle State"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, errResult := oke.Client(inputs)
	if errResult != nil {
		return errResult, nil
	}
	vnpid, err := oke.RequiredString("virtual_node_pool_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	vnid, err := oke.RequiredString("virtual_node_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	resp, err := client.GetVirtualNode(oke.Context(), okesdk.GetVirtualNodeRequest{VirtualNodePoolId: &vnpid, VirtualNodeId: &vnid})
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	vnode := oke.SummariseVirtualNode(&resp.VirtualNode)
	return oke.Result(fmt.Sprintf("Virtual node %q is %s", vnode["display_name"], vnode["lifecycle_state"]), map[string]interface{}{
		"virtual_node": vnode, "id": vnode["id"], "lifecycle_state": vnode["lifecycle_state"],
	}), nil
}
