// Package oracle_loadbalancer_load_balancer_delete deletes a load balancer by OCID.
// Deletion is asynchronous — returns a work-request id.
package oracle_loadbalancer_load_balancer_delete

import (
	"fmt"

	core "flomation.app/automate/executor"
	lbn "flomation.app/automate/executor/actions/oracle/loadbalancer"

	lb "github.com/oracle/oci-go-sdk/v65/loadbalancer"
)

const (
	Author       = "Dave McElin"
	Organisation = "Flomation"
	Name         = "OCI Load Balancer: Delete Load Balancer"
	Description  = "Permanently delete an Oracle Cloud load balancer by OCID. Deletion is asynchronous — returns a work-request id to poll with Get Work Request."
	Website      = "https://www.flomation.co"
	Icon         = "oracle+network-wired"
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
	{Name: "compartment_ocid", Type: core.ConnectionTypeString, Label: "Compartment OCID", Placeholder: "ocid1.compartment.oc1..aaaa… (scopes the load balancer picker)"},
	{Name: "load_balancer_ocid", Type: core.ConnectionTypeString, Label: "Load Balancer OCID", Placeholder: "ocid1.loadbalancer.oc1..aaaa…", Required: true},
}

var Outputs = [...]core.Connection{
	{Name: "tool_result", Type: core.ConnectionTypeString, Label: "Result summary"},
	{Name: "id", Type: core.ConnectionTypeString, Label: "Load Balancer OCID"},
	{Name: "work_request_id", Type: core.ConnectionTypeString, Label: "Work Request OCID"},
	{Name: "success", Type: core.ConnectionTypeBoolean, Label: "Success"},
	{Name: "error", Type: core.ConnectionTypeString, Label: "Error"},
}

func Execute(flow *core.Flow, node *core.Node, inputs []*core.Connection) (map[string]interface{}, error) {
	auth, client, id, errResult := lbn.ResourceClient(inputs, "load_balancer_ocid")
	if errResult != nil {
		return errResult, nil
	}
	resp, err := client.DeleteLoadBalancer(lbn.Context(), lb.DeleteLoadBalancerRequest{LoadBalancerId: &id})
	if err != nil {
		return lbn.ErrorResult(auth.OCIError(err)), nil
	}
	return map[string]interface{}{
		"tool_result":     fmt.Sprintf("Delete requested for load balancer %s — poll work request %s", id, lbn.Str(resp.OpcWorkRequestId)),
		"id":              id,
		"work_request_id": lbn.Str(resp.OpcWorkRequestId),
		"success":         true,
	}, nil
}
