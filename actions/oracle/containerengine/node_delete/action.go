// Package oracle_containerengine_node_delete deletes a single OKE worker node from its node
// pool by OCID, optionally shrinking the pool's desired size. Asynchronous — it returns a
// work-request id; poll Get Work Request until the deletion completes.
package oracle_containerengine_node_delete

import (
	core "flomation.app/automate/executor"
	oke "flomation.app/automate/executor/actions/oracle/containerengine"

	okesdk "github.com/oracle/oci-go-sdk/v65/containerengine"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Container Engine: Delete Node"
	Description  = "Delete a single Oracle Cloud OKE worker node from its node pool by OCID, optionally shrinking the pool. Asynchronous — returns a work-request id; poll Get Work Request until it completes."
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
	{Name: "node_pool_ocid", Type: core.ConnectionTypeString, Label: "Node Pool OCID", Placeholder: "ocid1.nodepool.oc1..aaaa… the node belongs to", Required: true},
	{Name: "node_ocid", Type: core.ConnectionTypeString, Label: "Node OCID", Placeholder: "ocid1.instance.oc1..aaaa… (the compute instance backing the node)", Required: true},
	{Name: "decrement_size", Type: core.ConnectionTypeBoolean, Label: "Decrement Pool Size", Placeholder: "Also shrink the node pool's desired size by one (optional)"},
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
	npID, err := oke.RequiredString("node_pool_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	nodeID, err := oke.RequiredString("node_ocid", inputs)
	if err != nil {
		return oke.ErrorResult(err.Error()), nil
	}
	req := okesdk.DeleteNodeRequest{NodePoolId: &npID, NodeId: &nodeID}
	if oke.BoolWasSet("decrement_size", inputs) {
		b := oke.OptionalBool("decrement_size", inputs, false)
		req.IsDecrementSize = &b
	}
	resp, err := client.DeleteNode(oke.Context(), req)
	if err != nil {
		return oke.ErrorResult(auth.OCIError(err)), nil
	}
	return oke.AsyncResult("Deleting node — poll Get Work Request until it completes", oke.Str(resp.OpcWorkRequestId)), nil
}
